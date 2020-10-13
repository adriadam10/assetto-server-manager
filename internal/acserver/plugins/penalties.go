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
)

type PenaltiesPlugin struct {
	server acserver.ServerPlugin
	logger acserver.Logger

	penalties []*penaltyInfo

	pitLane *pitlanedetection.PitLane
}

type penaltyInfo struct {
	carID    acserver.CarID
	warnings int

	totalCleanLapTime int64
	totalCleanLaps    int64
	totalLaps         int64
	clearPenaltyIn    int

	originalBallast    float32
	originalRestrictor float32

	driveThrough        bool
	driveThroughActive  bool
	driveThroughStarted time.Time
	driveThroughLaps    int
}

func NewPenaltiesPlugin(pitLane *pitlanedetection.PitLane) *PenaltiesPlugin {
	return &PenaltiesPlugin{
		pitLane: pitLane,
	}
}

func (p *PenaltiesPlugin) Init(server acserver.ServerPlugin, logger acserver.Logger) error {
	p.server = server
	p.logger = logger

	return nil
}

func (p *PenaltiesPlugin) OnNewConnection(car acserver.CarInfo) error {
	p.penalties = append(p.penalties, &penaltyInfo{
		carID:              car.CarID,
		originalBallast:    car.Ballast,
		originalRestrictor: car.Restrictor,
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
		// @TODO some error?
		return nil
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
	return nil
}

func (p *PenaltiesPlugin) OnEndSession(sessionFile string) error {
	resultsNeedModifying := false

	for _, penalty := range p.penalties {
		if penalty.driveThrough || penalty.clearPenaltyIn > 0 {
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
		if !penalty.driveThrough && penalty.clearPenaltyIn <= 0 {
			continue
		}

		averageCleanLap := time.Millisecond * time.Duration(float32(penalty.totalCleanLapTime)/float32(penalty.totalCleanLaps))

		if penalty.driveThrough {
			// driver finished session with unserved penalty, apply to results
			for _, result := range results.Result {
				if result.CarID == int(penalty.carID) {
					result.HasPenalty = true
					result.PenaltyTime += p.pitLane.AveragePitLaneTime + time.Second*10

					p.logger.Debugf("adding %s penalty to driver for unserved drive through penalty", result.PenaltyTime)

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
				if result.CarID == int(penalty.carID) {
					result.HasPenalty = true
					result.PenaltyTime += time.Second * 5 * time.Duration(penalty.clearPenaltyIn)

					p.logger.Debugf("adding %s penalty to driver for cutting the track", result.PenaltyTime)

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
	eventConfig := p.server.GetEventConfig()

	if eventConfig.CustomCutsEnabled && p.server.GetSessionInfo().SessionType == acserver.SessionTypeRace {
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
			// @TODO some error?
			return nil
		}

		penaltyInfo.totalLaps++

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

		if penaltyInfo.driveThroughLaps > -1 && penaltyInfo.driveThrough {
			penaltyInfo.driveThroughLaps--

			if penaltyInfo.driveThroughLaps == -1 {
				// @TODO probably send a chat to explain why kick first
				return p.server.KickUser(entrant.CarID, acserver.KickReasonGeneric)
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
			averageCleanLap = int64(float32(penaltyInfo.totalCleanLapTime) / float32(penaltyInfo.totalCleanLaps))
		}

		if eventConfig.CustomCutsIgnoreFirstLap && penaltyInfo.totalLaps == 1 {
			return nil
		}

		if ((penaltyInfo.totalCleanLaps == 0 && !eventConfig.CustomCutsOnlyIfCleanSet) || averageCleanLap > lap.LapTime.Milliseconds()) && lap.Cuts > 0 {
			penaltyInfo.warnings++

			if penaltyInfo.warnings > eventConfig.CustomCutsNumWarnings {
				var chatMessage string

				switch eventConfig.CustomCutsPenaltyType {
				case acserver.CutPenaltyKick:
					// @TODO probably send a chat to explain why kick first
					return p.server.KickUser(entrant.CarID, acserver.KickReasonGeneric)
				case acserver.CutPenaltyBallast:
					err = p.server.UpdateBoP(entrant.CarID, eventConfig.CustomCutsBoPAmount, entrant.Restrictor)

					if err != nil {
						p.logger.WithError(err).Error("Couldn't apply ballast from cuts")
						break
					}

					penaltyInfo.clearPenaltyIn += eventConfig.CustomCutsBoPNumLaps

					chatMessage = fmt.Sprintf("You have been given %.1fkg of ballast for %d laps for cutting the track", eventConfig.CustomCutsBoPAmount, eventConfig.CustomCutsBoPNumLaps)
				case acserver.CutPenaltyRestrictor:
					err = p.server.UpdateBoP(entrant.CarID, entrant.Ballast, eventConfig.CustomCutsBoPAmount)

					if err != nil {
						p.logger.WithError(err).Error("Couldn't apply restrictor from cuts")
						break
					}

					penaltyInfo.clearPenaltyIn += eventConfig.CustomCutsBoPNumLaps

					chatMessage = fmt.Sprintf("You have been given %.0f%% restrictor for %d laps for cutting the track", eventConfig.CustomCutsBoPAmount, eventConfig.CustomCutsBoPNumLaps)
				case acserver.CutPenaltyDriveThrough:
					if !penaltyInfo.driveThrough {
						penaltyInfo.driveThrough = true
						penaltyInfo.driveThroughLaps = eventConfig.CustomCutsDriveThroughNumLaps
					}

					chatMessage = fmt.Sprintf("You have been given a Drive Through Penalty for cutting! Please serve within the next %d lap(s).", penaltyInfo.driveThroughLaps)
				case acserver.CutPenaltyWarn:
					chatMessage = "Please avoid cutting the track! Your behaviour has been noted for admins to review in the results file"
				}

				err = p.server.SendChat(chatMessage, acserver.ServerCarID, entrant.CarID, true)

				if err != nil {
					p.logger.WithError(err).Error("Send chat returned an error")
				}

				penaltyInfo.warnings = 0
			} else {
				err = p.server.SendChat(fmt.Sprintf("You cut the track %d times this lap and gained time! (warning %d/%d)", lap.Cuts, penaltyInfo.warnings, eventConfig.CustomCutsNumWarnings), acserver.ServerCarID, entrant.CarID, true)

				if err != nil {
					p.logger.WithError(err).Error("Send chat returned an error")
				}
			}
		}
	}

	return nil
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
