package plugins

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/pitlanedetection"

	"github.com/sirupsen/logrus"
)

type PenaltiesPlugin struct {
	server      acserver.ServerPlugin
	logger      acserver.Logger
	eventConfig acserver.EventConfig

	penalties []*penaltyInfo

	pitLane  *pitlanedetection.PitLane
	drsZones []acserver.DRSZone
}

type penaltyInfo struct {
	carID              acserver.CarID
	warnings           int
	warningsDRS        int
	warningsCollisions int

	timePenalties []time.Duration

	totalCleanLapTime int64
	totalCleanLaps    int64
	totalLaps         int64
	clearPenaltyIn    int

	originalBallast    float32
	originalRestrictor float32

	penaltyBallast    float32
	penaltyRestrictor float32

	driveThrough        bool
	driveThroughActive  bool
	driveThroughStarted time.Time
	driveThroughLaps    int

	lastDRSActivationZoneTime time.Time
	lastDRSActivationZone     int
	isInDRSWindow             bool
	isAllowedDRS              bool
	drsIsOpen                 bool
	drsOpenedAt               time.Time
}

func NewPenaltiesPlugin(pitLane *pitlanedetection.PitLane) acserver.Plugin {
	return &PenaltiesPlugin{
		pitLane: pitLane,
	}
}

func (p *PenaltiesPlugin) Init(server acserver.ServerPlugin, logger acserver.Logger) error {
	p.server = server
	p.logger = logger

	return nil
}

func (p *PenaltiesPlugin) Shutdown() error {
	return nil
}

func (p *PenaltiesPlugin) OnNewConnection(car acserver.CarInfo) error {
	p.penalties = append(p.penalties, &penaltyInfo{
		carID:              car.CarID,
		originalBallast:    car.Ballast,
		originalRestrictor: car.Restrictor,

		lastDRSActivationZoneTime: time.Time{},
	})

	return nil
}

func (p *PenaltiesPlugin) OnConnectionClosed(car acserver.CarInfo) error {
	for i, penalty := range p.penalties {
		if penalty.carID == car.CarID {
			copy(p.penalties[i:], p.penalties[i+1:])
			p.penalties[len(p.penalties)-1] = nil
			p.penalties = p.penalties[:len(p.penalties)-1]

			break
		}
	}

	return nil
}

func (p *PenaltiesPlugin) OnCarUpdate(car acserver.CarInfo) error {
	if !p.pitLane.PitLaneCapable {
		return nil
	}

	leaderboard := p.server.GetLeaderboard()

	for _, line := range leaderboard {
		if line.Car.CarID == car.CarID {
			// don't mark penalties as completed if the car has already finished
			if line.Car.HasCompletedSession() {
				return nil
			}
		}
	}

	var penaltyInfo *penaltyInfo

	for _, penalty := range p.penalties {
		if penalty.carID == car.CarID {
			penaltyInfo = penalty
			break
		}
	}

	if penaltyInfo == nil {
		p.logger.Warnf("Car %d is connected, but has no penalty info!", car.CarID)

		return nil
	}

	// @TODO only from third lap?
	sessionInfo := p.server.GetSessionInfo()

	if p.eventConfig.DRSPenaltiesEnabled && sessionInfo.SessionType == acserver.SessionTypeRace {
		penaltyInfo.isAllowedDRS = false
		checkIfInWindow := false

		if int(sessionInfo.NumLaps) >= p.eventConfig.DRSPenaltiesEnableOnLap {
			for i, zone := range p.drsZones {
				// record all drs activation zone times
				if penaltyInfo.lastDRSActivationZoneTime.IsZero() || time.Since(penaltyInfo.lastDRSActivationZoneTime) > time.Second*5 {
					if car.PluginStatus.NormalisedSplinePos/zone.Detection > 0.97 && car.PluginStatus.NormalisedSplinePos/zone.Detection < 1.0 {
						penaltyInfo.lastDRSActivationZoneTime = time.Now()
						penaltyInfo.lastDRSActivationZone = i

						checkIfInWindow = true

						fmt.Println(fmt.Sprintf("%s crossed zone %d activation line at %s", car.Driver.Name, i, penaltyInfo.lastDRSActivationZoneTime.Format(time.Stamp)))
					}
				}
			}

			if checkIfInWindow {
				penaltyInfo.isInDRSWindow = false

				for _, penalty := range p.penalties {
					fmt.Println(penalty.carID, penalty.lastDRSActivationZone)
					if penalty.carID == penaltyInfo.carID || penalty.lastDRSActivationZone != penaltyInfo.lastDRSActivationZone {
						continue
					}

					diff := penaltyInfo.lastDRSActivationZoneTime.Sub(penalty.lastDRSActivationZoneTime)

					fmt.Println(fmt.Sprintf("%d crossed detection %s after %d (%t %t)", penaltyInfo.carID, diff.String(), penalty.carID, diff <= time.Second*2, diff > 0))

					if diff <= time.Second*2 && diff > 0 {
						penaltyInfo.isInDRSWindow = true
						fmt.Println(penaltyInfo.isInDRSWindow)
						break
					} else {
						penaltyInfo.isInDRSWindow = false
					}
				}
			}
		} else {
			penaltyInfo.isInDRSWindow = false
		}


		for i, zone := range p.drsZones {
			// only make sure drs is allowed if it's being used, once allowed don't then un-allow due to another zone
			if car.PluginStatus.StatusBytes&acserver.DRSByte == acserver.DRSByte && i == penaltyInfo.lastDRSActivationZone {
				fmt.Println(zone.Start, car.PluginStatus.NormalisedSplinePos, zone.End)
				isInZone := false

				if zone.Start < zone.End {
					// zone does not cross the start/finish line
					isInZone = car.PluginStatus.NormalisedSplinePos >= zone.Start && car.PluginStatus.NormalisedSplinePos <= zone.End
				} else {
					// zone does cross the start/finish line
					isInZone = (car.PluginStatus.NormalisedSplinePos >= zone.Start && car.PluginStatus.NormalisedSplinePos <= 1.0) || (car.PluginStatus.NormalisedSplinePos <= zone.End && car.PluginStatus.NormalisedSplinePos >= 0.0)
				}

				fmt.Println(fmt.Sprintf("Is in zone: %t, is in window: %t", isInZone, penaltyInfo.isInDRSWindow))

				if isInZone && penaltyInfo.isInDRSWindow {
					penaltyInfo.isAllowedDRS = true

					err := p.server.SendChat("DRS: OK", acserver.ServerCarID, penaltyInfo.carID, true)

					if err != nil {
						p.logger.WithError(err).Error("Send chat returned an error")
					}
					break
				}
			}
		}

		drsWasOpen := penaltyInfo.drsIsOpen

		if car.PluginStatus.StatusBytes&acserver.DRSByte == acserver.DRSByte {
			fmt.Println(fmt.Sprintf("%d is using DRS, is allowed: %t", penaltyInfo.carID, penaltyInfo.isAllowedDRS))

			penaltyInfo.drsIsOpen = true

			if penaltyInfo.drsIsOpen != drsWasOpen {
				penaltyInfo.drsOpenedAt = time.Now()
			}

			if !penaltyInfo.isAllowedDRS && time.Since(penaltyInfo.drsOpenedAt) >= time.Second {
				penaltyInfo.warningsDRS++

				if penaltyInfo.warningsDRS > p.eventConfig.DRSPenaltiesNumWarnings {
					p.applyPenalty(car, penaltyInfo, p.eventConfig.DRSPenaltiesPenaltyType, p.eventConfig.DRSPenaltiesBoPNumLaps, p.eventConfig.DRSPenaltiesDriveThroughNumLaps, p.eventConfig.DRSPenaltiesBoPAmount, "using DRS outside of the 2s window")

					penaltyInfo.warningsDRS = 0
				} else {
					err := p.server.SendChat(fmt.Sprintf("You used DRS whilst not within a 2s window! (warning %d/%d)", penaltyInfo.warningsDRS, p.eventConfig.DRSPenaltiesNumWarnings), acserver.ServerCarID, car.CarID, true)

					if err != nil {
						p.logger.WithError(err).Error("Send chat returned an error")
					}
				}
			}
		} else {
			penaltyInfo.drsIsOpen = false
			penaltyInfo.drsOpenedAt = time.Time{}
		}
	}

	if penaltyInfo.driveThrough {
		for _, pitLaneCar := range p.pitLane.Cars {
			if acserver.CarID(pitLaneCar.ID) == penaltyInfo.carID {
				if pitLaneCar.IsInPits {
					penaltyInfo.driveThroughActive = true
					if time.Since(penaltyInfo.driveThroughStarted) > p.pitLane.AveragePitLaneTime {
						penaltyInfo.driveThrough = false

						err := p.server.SendChat("Drive though complete!", acserver.ServerCarID, car.CarID, true)

						if err != nil {
							p.logger.WithError(err).Error("Send chat returned an error")
						}
					}
				} else {
					penaltyInfo.driveThroughActive = false
					penaltyInfo.driveThroughStarted = time.Now()
				}
			}
		}
	}

	return nil
}

func (p *PenaltiesPlugin) OnNewSession(newSession acserver.SessionInfo) error {
	p.eventConfig = p.server.GetEventConfig()

	for _, penalty := range p.penalties {
		if penalty.clearPenaltyIn > 0 {
			err := p.server.UpdateBoP(penalty.carID, penalty.originalBallast, penalty.originalRestrictor)

			if err != nil {
				p.logger.WithError(err).Error("Couldn't reset BoP to original")
			} else {
				err = p.server.SendChat("New session started, your penalty BoP has been cleared", acserver.ServerCarID, penalty.carID, true)

				if err != nil {
					p.logger.WithError(err).Error("Send chat returned an error")
				}
			}
		}

		penalty.driveThrough = false
		penalty.clearPenaltyIn = 0
		penalty.warnings = 0
		penalty.totalLaps = 0
		penalty.totalCleanLaps = 0
		penalty.totalCleanLapTime = 0
	}

	if !p.pitLane.PitLaneCapable && p.eventConfig.CustomCutsPenaltyType == acserver.CutPenaltyDriveThrough {
		logrus.Warn("New session is configured to give drive through penalties but the track is not capable of pit lane detection! " +
			"Please make sure the track has fast_lane.ai and pit_lane.ai files for each layout and re-upload it to the manager! " +
			"For this session drivers will be given time penalties in place of drive through penalties.")
	}

	if p.eventConfig.DRSPenaltiesEnabled {
		//@TODO install path?
		trackPath := filepath.Join("assetto", "content", "tracks", p.eventConfig.Track)

		if p.eventConfig.TrackLayout != "" {
			trackPath = filepath.Join(trackPath, p.eventConfig.TrackLayout)
		}

		//@TODO drs zone filename can be fallback, or the ignore drs zones one
		drsZones, err := acserver.LoadDRSZones(filepath.Join(trackPath, "data", "drs_zones.ini"))

		if err != nil {
			logrus.WithError(err).Warnf("New session is configured with DRS Penalties, but the track drs_zones.ini file is missing from %s!", filepath.Join("assetto", "content", "tracks", "data", "drs_zones.ini"))
		} else {
			for _, zone := range drsZones {
				p.drsZones = append(p.drsZones, zone)
			}
		}
	}

	return nil
}

func (p *PenaltiesPlugin) OnEndSession(sessionFile string) error {
	resultsNeedModifying := false

	for _, penalty := range p.penalties {
		if penalty.driveThrough || penalty.clearPenaltyIn > 0 || len(penalty.timePenalties) > 1 {
			resultsNeedModifying = true
		}
	}

	if !resultsNeedModifying {
		return nil
	}

	var results acserver.SessionResults

	path := filepath.Join("assetto", "results", sessionFile)

	data, err := ioutil.ReadFile(path)

	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &results)

	if err != nil {
		return err
	}

	for _, penalty := range p.penalties {
		if !penalty.driveThrough && penalty.clearPenaltyIn <= 0 && len(penalty.timePenalties) == 0 {
			continue
		}

		averageCleanLap := time.Millisecond * time.Duration(float32(penalty.totalCleanLapTime)/float32(penalty.totalCleanLaps))

		if penalty.driveThrough {
			// driver finished session with unserved penalty, apply to results
			for _, result := range results.Result {
				if result.CarID == penalty.carID {
					result.HasPenalty = true
					result.PenaltyTime += p.pitLane.AveragePitLaneTime + time.Second*10

					p.logger.Infof("adding %s penalty to car %d for unserved drive through penalty", result.CarID, result.PenaltyTime)

					if penalty.totalCleanLaps != 0 {
						if result.PenaltyTime > averageCleanLap {
							result.LapPenalty = int(result.PenaltyTime / averageCleanLap)
						}
					}
				}
			}
		}

		if penalty.clearPenaltyIn > 0 {
			// driver still had a penalty for some laps, time penalty instead
			for _, result := range results.Result {
				if result.CarID == penalty.carID {
					result.HasPenalty = true
					result.PenaltyTime += time.Second * 5 * time.Duration(penalty.clearPenaltyIn)

					p.logger.Infof("adding %s penalty to car %d for cutting the track, based on remaining laps of BoP at session end", result.CarID, result.PenaltyTime)

					if penalty.totalCleanLaps != 0 {
						if result.PenaltyTime > averageCleanLap {
							result.LapPenalty = int(result.PenaltyTime / averageCleanLap)
						}
					}
				}
			}
		}

		for _, timePenalty := range penalty.timePenalties {
			for _, result := range results.Result {
				if result.CarID == penalty.carID {
					result.HasPenalty = true
					result.PenaltyTime += timePenalty

					p.logger.Infof("adding %s penalty to car %d for cutting the track", result.CarID, result.PenaltyTime)

					if penalty.totalCleanLaps != 0 {
						if result.PenaltyTime > averageCleanLap {
							result.LapPenalty = int(result.PenaltyTime / averageCleanLap)
						}
					}
				}
			}
		}
	}

	file, err := os.Create(path)

	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "\t")

	return encoder.Encode(results)
}

func (p *PenaltiesPlugin) OnVersion(version uint16) error {
	return nil
}

func (p *PenaltiesPlugin) OnChat(chat acserver.Chat) error {
	return nil
}

func (p *PenaltiesPlugin) OnClientLoaded(car acserver.CarInfo) error {
	return nil
}

func (p *PenaltiesPlugin) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {
	if p.eventConfig.CustomCutsEnabled && p.server.GetSessionInfo().SessionType == acserver.SessionTypeRace {
		var averageCleanLap int64

		entrant, err := p.server.GetCarInfo(carID)

		if err != nil {
			return err
		}

		var penaltyInfo *penaltyInfo

		for _, penalty := range p.penalties {
			if penalty.carID == carID {
				penaltyInfo = penalty
				break
			}
		}

		if penaltyInfo == nil {
			logrus.Warnf("Car %d completed a lap, but is not present in the penalty info list, penalties disabled for this driver", carID)

			return nil
		}

		penaltyInfo.totalLaps++

		if penaltyInfo.clearPenaltyIn > 0 {
			penaltyInfo.clearPenaltyIn--

			// @TODO needs to be per penalty
			if penaltyInfo.clearPenaltyIn == 0 {
				switch p.eventConfig.CustomCutsPenaltyType {
				case acserver.CutPenaltyBallast:
					penaltyInfo.penaltyBallast -= p.eventConfig.CustomCutsBoPAmount
				case acserver.CutPenaltyRestrictor:
					penaltyInfo.penaltyRestrictor -= p.eventConfig.CustomCutsBoPAmount
				}

				ballastToApply := penaltyInfo.originalBallast
				restrictorToApply := penaltyInfo.originalRestrictor

				if penaltyInfo.penaltyBallast > ballastToApply {
					ballastToApply = penaltyInfo.penaltyBallast
				}

				if penaltyInfo.penaltyRestrictor > restrictorToApply {
					restrictorToApply = penaltyInfo.penaltyRestrictor
				}

				err = p.server.UpdateBoP(entrant.CarID, ballastToApply, restrictorToApply)

				if err != nil {
					p.logger.WithError(err).Error("Couldn't reset BoP to original")
				} else {
					err = p.server.SendChat("Your penalty BoP has been cleared", acserver.ServerCarID, entrant.CarID, true)

					if err != nil {
						p.logger.WithError(err).Error("Send chat returned an error")
					}
				}

			}
		}

		if penaltyInfo.driveThroughLaps > -1 && penaltyInfo.driveThrough {
			penaltyInfo.driveThroughLaps--

			if penaltyInfo.driveThroughLaps == -1 {
				err = p.server.SendChat("You have been kicked for cutting the track", acserver.ServerCarID, entrant.CarID, true)

				if err != nil {
					p.logger.WithError(err).Error("Send chat returned an error")
				}

				go func() {
					time.Sleep(time.Second * 10)

					_ = p.server.KickUser(entrant.CarID, acserver.KickReasonGeneric)
				}()

				return nil
			}

			err = p.server.SendChat(fmt.Sprintf("You have %d laps left to serve your drive through penalty", penaltyInfo.driveThroughLaps), acserver.ServerCarID, entrant.CarID, true)

			if err != nil {
				p.logger.WithError(err).Error("Send chat returned an error")
			}
		}

		if lap.Cuts == 0 {
			penaltyInfo.totalCleanLaps++
			penaltyInfo.totalCleanLapTime += lap.LapTime.Milliseconds()
		}

		if penaltyInfo.totalCleanLaps != 0 {
			// @TODO remove laps 107% (or so - figure it out) slower than the best from the average set
			averageCleanLap = int64(float32(penaltyInfo.totalCleanLapTime) / float32(penaltyInfo.totalCleanLaps))
		}

		if p.eventConfig.CustomCutsIgnoreFirstLap && penaltyInfo.totalLaps == 1 {
			return nil
		}

		if ((penaltyInfo.totalCleanLaps == 0 && !p.eventConfig.CustomCutsOnlyIfCleanSet) || averageCleanLap > lap.LapTime.Milliseconds()) && lap.Cuts > 0 {
			penaltyInfo.warnings++

			if penaltyInfo.warnings > p.eventConfig.CustomCutsNumWarnings {
				p.applyPenalty(entrant, penaltyInfo, p.eventConfig.CustomCutsPenaltyType, p.eventConfig.CustomCutsBoPNumLaps, p.eventConfig.CustomCutsDriveThroughNumLaps, p.eventConfig.CustomCutsBoPAmount, "cutting the track")

				penaltyInfo.warnings = 0
			} else {
				err = p.server.SendChat(fmt.Sprintf("You cut the track %d times this lap and gained time! (warning %d/%d)", lap.Cuts, penaltyInfo.warnings, p.eventConfig.CustomCutsNumWarnings), acserver.ServerCarID, entrant.CarID, true)

				if err != nil {
					p.logger.WithError(err).Error("Send chat returned an error")
				}
			}
		}
	}

	return nil
}

func (p *PenaltiesPlugin) applyPenalty(entrant acserver.CarInfo, penaltyInfo *penaltyInfo, penaltyType acserver.PenaltyType, clearPenaltyIn, driveThroughNumLaps int, bopAmount float32, messageContext string) {
	var chatMessage string
	var err error

	switch penaltyType {
	case acserver.CutPenaltyKick:
		err = p.server.SendChat(fmt.Sprintf("You have been kicked for %s", messageContext), acserver.ServerCarID, entrant.CarID, true)

		if err != nil {
			p.logger.WithError(err).Error("Send chat returned an error")
		}

		go func() {
			time.Sleep(time.Second * 10)

			_ = p.server.KickUser(entrant.CarID, acserver.KickReasonGeneric)
		}()

		return
	case acserver.CutPenaltyBallast:
		penaltyInfo.penaltyBallast += bopAmount

		err = p.server.UpdateBoP(entrant.CarID, penaltyInfo.penaltyBallast, entrant.Restrictor)

		if err != nil {
			p.logger.WithError(err).Errorf("Couldn't apply ballast for %s", messageContext)
			break
		}

		penaltyInfo.clearPenaltyIn += clearPenaltyIn

		chatMessage = fmt.Sprintf("You have been given %.1fkg of ballast for %d laps for %s", bopAmount, clearPenaltyIn, messageContext)
	case acserver.CutPenaltyRestrictor:
		penaltyInfo.penaltyRestrictor += bopAmount

		err = p.server.UpdateBoP(entrant.CarID, entrant.Ballast, penaltyInfo.penaltyRestrictor)

		if err != nil {
			p.logger.WithError(err).Errorf("Couldn't apply restrictor for %s", messageContext)
			break
		}

		penaltyInfo.clearPenaltyIn += clearPenaltyIn

		chatMessage = fmt.Sprintf("You have been given %.0f%% restrictor for %d laps for %s", bopAmount, clearPenaltyIn, messageContext)
	case acserver.CutPenaltyDriveThrough:
		if !p.pitLane.PitLaneCapable {
			penaltyInfo.timePenalties = append(penaltyInfo.timePenalties, time.Second*time.Duration(20))

			chatMessage = fmt.Sprintf("You have been given a %d second penalty for %s", 20, messageContext)
		} else {
			if !penaltyInfo.driveThrough {
				penaltyInfo.driveThrough = true
				penaltyInfo.driveThroughLaps = driveThroughNumLaps
			}

			chatMessage = fmt.Sprintf("You have been given a Drive Through Penalty %s! Please serve within the next %d lap(s).", messageContext, penaltyInfo.driveThroughLaps)
		}
	case acserver.CutPenaltyWarn:
		chatMessage = fmt.Sprintf("Please avoid %s! Your behaviour has been noted for admins to review in the results file", messageContext)
	}

	err = p.server.SendChat(chatMessage, acserver.ServerCarID, entrant.CarID, true)

	if err != nil {
		p.logger.WithError(err).Error("Send chat returned an error")
	}
}

func (p *PenaltiesPlugin) OnClientEvent(_ acserver.ClientEvent) error {
	return nil
}

func (p *PenaltiesPlugin) OnCollisionWithCar(event acserver.ClientEvent) error {
	return nil
}

func (p *PenaltiesPlugin) OnCollisionWithEnv(event acserver.ClientEvent) error {
	return nil
}

func (p *PenaltiesPlugin) OnSectorCompleted(car acserver.CarInfo, split acserver.Split) error {
	return nil
}

func (p *PenaltiesPlugin) OnWeatherChange(_ acserver.CurrentWeather) error {
	return nil
}

func (p *PenaltiesPlugin) OnTyreChange(car acserver.CarInfo, tyres string) error {
	return nil
}

func (p *PenaltiesPlugin) SortLeaderboard(_ acserver.SessionType, _ []*acserver.LeaderboardLine) {

}
