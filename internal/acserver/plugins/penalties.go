package plugins

import (
	"fmt"

	"justapengu.in/acsm/internal/acserver"
)

type PenaltiesPlugin struct {
	server acserver.ServerPlugin
	logger acserver.Logger

	penalties []*penaltyInfo
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
}

func NewPenaltiesPlugin() *PenaltiesPlugin {
	return &PenaltiesPlugin{}
}

func (p *PenaltiesPlugin) Init(server acserver.ServerPlugin, logger acserver.Logger) error {
	p.server = server
	p.logger = logger

	return nil
}

func (p *PenaltiesPlugin) OnNewConnection(car acserver.Car) error {
	p.penalties = append(p.penalties, &penaltyInfo{
		carID:              car.CarID,
		originalBallast:    car.Ballast,
		originalRestrictor: car.Restrictor,
	})

	return nil
}

func (p *PenaltiesPlugin) OnConnectionClosed(car acserver.Car) error {
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

func (p *PenaltiesPlugin) OnCarUpdate(car acserver.Car) error {
	return nil
}

func (p *PenaltiesPlugin) OnNewSession(newSession acserver.SessionInfo) error {
	return nil
}

func (p *PenaltiesPlugin) OnEndSession(sessionFile string) error {
	return nil
}

func (p *PenaltiesPlugin) OnVersion(version uint16) error {
	return nil
}

func (p *PenaltiesPlugin) OnChat(chat acserver.Chat) error {
	return nil
}

func (p *PenaltiesPlugin) OnClientLoaded(car acserver.Car) error {
	return nil
}

func (p *PenaltiesPlugin) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {
	eventConfig := p.server.GetEventConfig()

	// @TODO remove this, implement to custom race form
	if !eventConfig.CustomCutsEnabled {
		eventConfig.CustomCutsEnabled = true
		eventConfig.CustomCutsNumWarnings = 2
		eventConfig.CustomCutsPenaltyType = acserver.CutPenaltyBallast
		eventConfig.CustomCutsBoPAmount = 100
		eventConfig.CustomCutsBoPNumLaps = 2
		eventConfig.CustomCutsIgnoreFirstLap = true // @TODO test
	}

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
					err = p.server.SendChat("Your penalty BoP has been cleared", acserver.ServerCarID, entrant.CarID)

					if err != nil {
						p.logger.WithError(err).Error("Send chat returned an error")
					}
				}

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

					chatMessage = fmt.Sprintf("You have been given %.1fkg of ballast for cutting the track", eventConfig.CustomCutsBoPAmount)
				case acserver.CutPenaltyRestrictor:
					err = p.server.UpdateBoP(entrant.CarID, entrant.Ballast, eventConfig.CustomCutsBoPAmount)

					if err != nil {
						p.logger.WithError(err).Error("Couldn't apply restrictor from cuts")
						break
					}

					penaltyInfo.clearPenaltyIn += eventConfig.CustomCutsBoPNumLaps

					chatMessage = fmt.Sprintf("You have been given %.0f%% restrictor for cutting the track", eventConfig.CustomCutsBoPAmount)
				//case acserver.CutPenaltyDriveThrough:
					// @TODO maybe, 2 laps to complete, on fail kick, if only 2 laps remaining add 30s to race time
				case acserver.CutPenaltyWarn:
					chatMessage = "Please avoid cutting the track! Your behaviour has been noted for admins to review in the results file"
				}

				err = p.server.SendChat(chatMessage, acserver.ServerCarID, entrant.CarID)

				if err != nil {
					p.logger.WithError(err).Error("Send chat returned an error")
				}

				penaltyInfo.warnings = 0
			} else {
				err = p.server.SendChat(fmt.Sprintf("You cut the track %d times this lap and gained time! (warning %d/%d)", lap.Cuts, penaltyInfo.warnings, eventConfig.CustomCutsNumWarnings), acserver.ServerCarID, entrant.CarID)

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

func (p *PenaltiesPlugin) OnSectorCompleted(split acserver.Split) error {
	return nil
}

func (p *PenaltiesPlugin) OnWeatherChange(_ acserver.CurrentWeather) error {
	return nil
}

func (p *PenaltiesPlugin) OnTyreChange(car acserver.Car, tyres string) error {
	return nil
}
