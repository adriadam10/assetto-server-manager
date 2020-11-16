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

	pitLane          *pitlanedetection.PitLane
	drsZones         []acserver.DRSZone
	installPath      string
	drsZonesFilename string
}

type penaltyInfo struct {
	carID              acserver.CarID
	warnings           int
	warningsDRS        int
	warningsCollisions int

	timePenalties []time.Duration

	cleanLaps         []int64
	bestLap           int64
	totalCleanLapTime int64
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

	collisionRateLimiter time.Time
}

func NewPenaltiesPlugin(pitLane *pitlanedetection.PitLane, installPath, drsZonesFilename string) acserver.Plugin {
	return &PenaltiesPlugin{
		pitLane:          pitLane,
		installPath:      installPath,
		drsZonesFilename: drsZonesFilename,
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

	sessionInfo := p.server.GetSessionInfo()

	if p.eventConfig.DRSPenaltiesEnabled && sessionInfo.SessionType == acserver.SessionTypeRace {
		penaltyInfo.isAllowedDRS = false
		checkIfInWindow := false

		if int(sessionInfo.NumLaps) >= p.eventConfig.DRSPenaltiesEnableOnLap {
			for i, zone := range p.drsZones {
				// record all drs activation zone times
				if penaltyInfo.lastDRSActivationZoneTime.IsZero() || time.Since(penaltyInfo.lastDRSActivationZoneTime) > time.Second*5 {
					splineRatioToDetectionZone := car.PluginStatus.NormalisedSplinePos / zone.Detection

					if splineRatioToDetectionZone > 0.97 && splineRatioToDetectionZone < 1.0 {
						penaltyInfo.lastDRSActivationZoneTime = time.Now()
						penaltyInfo.lastDRSActivationZone = i

						checkIfInWindow = true
					}
				}
			}

			if checkIfInWindow {
				penaltyInfo.isInDRSWindow = false

				for _, penalty := range p.penalties {
					if penalty.carID == penaltyInfo.carID || penalty.lastDRSActivationZone != penaltyInfo.lastDRSActivationZone {
						continue
					}

					diff := penaltyInfo.lastDRSActivationZoneTime.Sub(penalty.lastDRSActivationZoneTime)

					if diff <= time.Second*time.Duration(p.eventConfig.DRSPenaltiesWindow) && diff > 0 {
						penaltyInfo.isInDRSWindow = true
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
				isInZone := false

				if zone.Start < zone.End {
					// zone does not cross the start/finish line
					isInZone = car.PluginStatus.NormalisedSplinePos >= zone.Start && car.PluginStatus.NormalisedSplinePos <= zone.End
				} else {
					// zone does cross the start/finish line
					isInZone = (car.PluginStatus.NormalisedSplinePos >= zone.Start && car.PluginStatus.NormalisedSplinePos <= 1.0) || (car.PluginStatus.NormalisedSplinePos <= zone.End && car.PluginStatus.NormalisedSplinePos >= 0.0)
				}

				if isInZone && penaltyInfo.isInDRSWindow {
					penaltyInfo.isAllowedDRS = true

					break
				}
			}
		}

		drsWasOpen := penaltyInfo.drsIsOpen

		if car.PluginStatus.StatusBytes&acserver.DRSByte == acserver.DRSByte {

			penaltyInfo.drsIsOpen = true

			if penaltyInfo.drsIsOpen != drsWasOpen {
				penaltyInfo.drsOpenedAt = time.Now()
			}

			if !penaltyInfo.isAllowedDRS && time.Since(penaltyInfo.drsOpenedAt) >= time.Second {
				penaltyInfo.warningsDRS++

				if penaltyInfo.warningsDRS > p.eventConfig.DRSPenaltiesNumWarnings {
					p.applyPenalty(car, penaltyInfo, p.eventConfig.DRSPenaltiesPenaltyType, p.eventConfig.DRSPenaltiesBoPNumLaps, p.eventConfig.DRSPenaltiesDriveThroughNumLaps, p.eventConfig.DRSPenaltiesBoPAmount, fmt.Sprintf("using DRS outside of the %.0fs window", p.eventConfig.DRSPenaltiesWindow))

					penaltyInfo.warningsDRS = 0
				} else {
					err := p.server.SendChat(fmt.Sprintf("You used DRS whilst not within the %.1fs window! (warning %d/%d)", p.eventConfig.DRSPenaltiesWindow, penaltyInfo.warningsDRS, p.eventConfig.DRSPenaltiesNumWarnings), acserver.ServerCarID, car.CarID, true)

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
		penalty.cleanLaps = nil
		penalty.totalCleanLapTime = 0
	}

	if !p.pitLane.PitLaneCapable && p.eventConfig.CustomCutsPenaltyType == acserver.PenaltyDriveThrough {
		logrus.Warn("New session is configured to give drive through penalties but the track is not capable of pit lane detection! " +
			"Please make sure the track has fast_lane.ai and pit_lane.ai files for each layout and re-upload it to the manager! " +
			"For this session drivers will be given time penalties in place of drive through penalties.")
	}

	if p.eventConfig.DRSPenaltiesEnabled {
		trackPath := filepath.Join(p.installPath, "content", "tracks", p.eventConfig.Track)

		if p.eventConfig.TrackLayout != "" {
			trackPath = filepath.Join(trackPath, p.eventConfig.TrackLayout)
		}

		drsZones, err := acserver.LoadDRSZones(filepath.Join(trackPath, "data", p.drsZonesFilename))

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

		numCleanLaps := len(penalty.cleanLaps)

		averageCleanLap := time.Millisecond * time.Duration(float32(penalty.totalCleanLapTime)/float32(numCleanLaps))

		if penalty.driveThrough {
			// driver finished session with unserved penalty, apply to results
			for _, result := range results.Result {
				if result.CarID == penalty.carID {
					result.HasPenalty = true
					result.PenaltyTime += p.pitLane.AveragePitLaneTime + time.Second*10

					p.logger.Infof("adding %s penalty to car %d for unserved drive through penalty", result.CarID, result.PenaltyTime)

					if numCleanLaps != 0 {
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

					if numCleanLaps != 0 {
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

					if numCleanLaps != 0 {
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
	if p.server.GetSessionInfo().SessionType != acserver.SessionTypeRace {
		return nil
	}

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

	if int(penaltyInfo.totalLaps) == p.eventConfig.DRSPenaltiesEnableOnLap {
		err = p.server.SendChat("DRS Enabled", acserver.ServerCarID, entrant.CarID, true)

		if err != nil {
			p.logger.WithError(err).Error("Send chat returned an error")
		}
	}

	if lap.Cuts == 0 {
		lapTime := lap.LapTime.Milliseconds()

		if float32(lapTime) <= float32(penaltyInfo.bestLap)*1.07 || penaltyInfo.bestLap == 0 {
			penaltyInfo.cleanLaps = append(penaltyInfo.cleanLaps, lapTime)

			if lapTime < penaltyInfo.bestLap || penaltyInfo.bestLap == 0 {
				penaltyInfo.bestLap = lapTime

				// remove any exiting laps >107% slower than the new best
				for i, lap := range penaltyInfo.cleanLaps {
					if float32(lap) > float32(penaltyInfo.bestLap)*1.07 {
						penaltyInfo.cleanLaps = append(penaltyInfo.cleanLaps[:i], penaltyInfo.cleanLaps[i+1:]...)
					}
				}
			}
		}
	}

	penaltyInfo.totalCleanLapTime = 0

	for _, lap := range penaltyInfo.cleanLaps {
		penaltyInfo.totalCleanLapTime += lap
	}

	numCleanLaps := len(penaltyInfo.cleanLaps)

	if numCleanLaps != 0 {
		averageCleanLap = int64(float32(penaltyInfo.totalCleanLapTime) / float32(numCleanLaps))
	}

	if p.eventConfig.CustomCutsIgnoreFirstLap && penaltyInfo.totalLaps == 1 {
		return nil
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

	if penaltyInfo.clearPenaltyIn > 0 {
		penaltyInfo.clearPenaltyIn--

		if penaltyInfo.clearPenaltyIn == 0 {
			err = p.server.UpdateBoP(entrant.CarID, penaltyInfo.originalBallast, penaltyInfo.originalRestrictor)

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

	if p.eventConfig.CustomCutsEnabled {
		if ((numCleanLaps == 0 && !p.eventConfig.CustomCutsOnlyIfCleanSet) || averageCleanLap > lap.LapTime.Milliseconds()) && lap.Cuts > 0 {
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
	case acserver.PenaltyKick:
		err = p.server.SendChat(fmt.Sprintf("You have been kicked for %s", messageContext), acserver.ServerCarID, entrant.CarID, true)

		if err != nil {
			p.logger.WithError(err).Error("Send chat returned an error")
		}

		go func() {
			time.Sleep(time.Second * 10)

			_ = p.server.KickUser(entrant.CarID, acserver.KickReasonGeneric)
		}()

		return
	case acserver.PenaltyBallast:
		penaltyInfo.penaltyBallast += bopAmount

		err = p.server.UpdateBoP(entrant.CarID, penaltyInfo.penaltyBallast, entrant.Restrictor)

		if err != nil {
			p.logger.WithError(err).Errorf("Couldn't apply ballast for %s", messageContext)
			break
		}

		penaltyInfo.clearPenaltyIn += clearPenaltyIn

		chatMessage = fmt.Sprintf("You have been given %.1fkg of ballast for %d laps for %s", bopAmount, clearPenaltyIn, messageContext)
	case acserver.PenaltyRestrictor:
		penaltyInfo.penaltyRestrictor += bopAmount

		err = p.server.UpdateBoP(entrant.CarID, entrant.Ballast, penaltyInfo.penaltyRestrictor)

		if err != nil {
			p.logger.WithError(err).Errorf("Couldn't apply restrictor for %s", messageContext)
			break
		}

		penaltyInfo.clearPenaltyIn += clearPenaltyIn

		chatMessage = fmt.Sprintf("You have been given %.0f%% restrictor for %d laps for %s", bopAmount, clearPenaltyIn, messageContext)
	case acserver.PenaltyDriveThrough:
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
	case acserver.PenaltyWarn:
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
	return p.onCollision(event, true)
}

func (p *PenaltiesPlugin) OnCollisionWithEnv(event acserver.ClientEvent) error {
	return p.onCollision(event, false)
}

func (p *PenaltiesPlugin) onCollision(event acserver.ClientEvent, withCar bool) error {
	if !p.eventConfig.CollisionPenaltiesEnabled {
		return nil
	}

	var penaltyInfo *penaltyInfo

	for _, penalty := range p.penalties {
		if penalty.carID == event.CarID {
			penaltyInfo = penalty
			break
		}
	}

	if penaltyInfo == nil {
		logrus.Warnf("Car %d was in a collision, but is not present in the penalty info list, penalties disabled for this driver", event.CarID)

		return nil
	}

	collisionTime := time.Now()

	// AC often reports multiple collisions for one "collision" if cars bounce a bit etc.
	// so we add a bit of leniency
	if !collisionTime.After(penaltyInfo.collisionRateLimiter.Add(time.Second * 2)) {
		logrus.Debugf("Car %d collision was ignored by penalty system due to collision debouncing", event.CarID)

		return nil
	}

	penaltyInfo.collisionRateLimiter = time.Now()

	entrant, err := p.server.GetCarInfo(penaltyInfo.carID)

	if err != nil {
		return err
	}

	sessionInfo := p.server.GetSessionInfo()

	if sessionInfo.NumLaps <= 1 && p.eventConfig.CollisionPenaltiesIgnoreFirstLap {
		return nil
	}

	if event.Speed > p.eventConfig.CollisionPenaltiesOnlyOverSpeed {
		penaltyInfo.warningsCollisions++

		if penaltyInfo.warningsCollisions > p.eventConfig.CollisionPenaltiesNumWarnings {
			p.applyPenalty(entrant, penaltyInfo, p.eventConfig.CollisionPenaltiesPenaltyType, p.eventConfig.CollisionPenaltiesBoPNumLaps, p.eventConfig.CollisionPenaltiesDriveThroughNumLaps, p.eventConfig.CollisionPenaltiesBoPAmount, "being involved in collisions")

			penaltyInfo.warningsCollisions = 0
		} else {
			context := "the environment"

			if withCar {
				context = "another driver"
			}

			err = p.server.SendChat(fmt.Sprintf("You collided with "+context+"! (warning %d/%d)", penaltyInfo.warningsCollisions, p.eventConfig.CollisionPenaltiesNumWarnings), acserver.ServerCarID, entrant.CarID, true)

			if err != nil {
				p.logger.WithError(err).Error("Send chat returned an error")
			}
		}
	}

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
