package acsm

import (
	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/udp"
)

type UDPPluginAdapter struct {
	raceManager           *RaceManager
	raceControl           *RaceControl
	championshipManager   *ChampionshipManager
	raceWeekendManager    *RaceWeekendManager
	contentManagerWrapper *ContentManagerWrapper
	server                acserver.ServerPlugin
}

func NewUDPPluginAdapter(raceManager *RaceManager, raceControl *RaceControl, championshipManager *ChampionshipManager, raceWeekendManager *RaceWeekendManager, contentManagerWrapper *ContentManagerWrapper) *UDPPluginAdapter {
	return &UDPPluginAdapter{
		raceManager:           raceManager,
		raceControl:           raceControl,
		championshipManager:   championshipManager,
		raceWeekendManager:    raceWeekendManager,
		contentManagerWrapper: contentManagerWrapper,
	}
}

func (r *UDPPluginAdapter) UDPCallback(message udp.Message) {
	if !config.Server.PerformanceMode {
		r.raceControl.UDPCallback(message)
	}

	if message.Event() != udp.EventCarUpdate {
		r.championshipManager.ChampionshipEventCallback(message)
		r.raceWeekendManager.UDPCallback(message)
		r.raceManager.LoopCallback(message)
		r.contentManagerWrapper.UDPCallback(message)
	}
}

func (r *UDPPluginAdapter) Init(server acserver.ServerPlugin, _ acserver.Logger) error {
	r.server = server
	r.raceControl.server = server
	r.server.SetUpdateInterval(RealtimePosInterval)

	return nil
}

func (r *UDPPluginAdapter) OnVersion(version uint16) error {
	r.UDPCallback(udp.Version(version))

	return nil
}

func convertSessionInfoToUDP(eventType udp.Event, session acserver.SessionInfo) udp.SessionInfo {
	return udp.SessionInfo{
		Version:             session.Version,
		SessionIndex:        session.SessionIndex,
		CurrentSessionIndex: session.SessionIndex,
		SessionCount:        session.SessionCount,
		ServerName:          session.ServerName,
		Track:               session.Track,
		TrackConfig:         session.TrackConfig,
		Name:                session.Name,
		Type:                session.SessionType,
		Time:                session.NumMinutes,
		Laps:                session.NumLaps,
		WaitTime:            uint16(session.WaitTime),
		AmbientTemp:         session.AmbientTemp,
		RoadTemp:            session.RoadTemp,
		WeatherGraphics:     session.WeatherGraphics,
		ElapsedMilliseconds: int32(session.ElapsedTime.Milliseconds()),
		IsSolo:              session.IsSolo,
		EventType:           eventType,
	}
}

func (r *UDPPluginAdapter) OnNewSession(newSession acserver.SessionInfo) error {
	r.UDPCallback(convertSessionInfoToUDP(udp.EventNewSession, newSession))

	return nil
}

func (r *UDPPluginAdapter) OnWeatherChange(_ acserver.CurrentWeather) error {
	r.UDPCallback(convertSessionInfoToUDP(udp.EventSessionInfo, r.server.GetSessionInfo()))

	return nil
}

func (r *UDPPluginAdapter) OnEndSession(sessionFile string) error {
	r.UDPCallback(udp.EndSession(sessionFile))

	return nil
}

func (r *UDPPluginAdapter) OnNewConnection(car acserver.Car) error {
	r.UDPCallback(udp.SessionCarInfo{
		CarID:      car.CarID,
		DriverName: car.Driver.Name,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		CarModel:   car.Model,
		CarSkin:    car.Skin,
		EventType:  udp.EventNewConnection,
	})

	return nil
}

func (r *UDPPluginAdapter) OnClientLoaded(car acserver.Car) error {
	r.UDPCallback(udp.ClientLoaded(car.CarID))

	return nil
}

func (r *UDPPluginAdapter) OnSectorCompleted(split acserver.Split) error {
	r.UDPCallback(udp.SplitCompleted{
		CarID: split.Car.CarID,
		Index: split.Index,
		Time:  split.Time,
		Cuts:  split.Cuts,
	})

	return nil
}

func (r *UDPPluginAdapter) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {
	leaderboard := r.server.GetLeaderboard()

	l := udp.LapCompleted{
		CarID:     carID,
		LapTime:   uint32(lap.LapTime.Milliseconds()),
		Cuts:      uint8(lap.Cuts),
		CarsCount: uint8(len(leaderboard)),
	}

	for _, line := range leaderboard {
		completed := uint8(0)

		if line.Car.SessionData.HasCompletedSession {
			completed = 1
		}

		l.Cars = append(l.Cars, &udp.LapCompletedCar{
			CarID:     line.Car.CarID,
			LapTime:   uint32(line.Time.Milliseconds()),
			Laps:      uint16(line.NumLaps),
			Completed: completed,
		})
	}

	r.UDPCallback(l)

	return nil
}

func (r *UDPPluginAdapter) OnCarUpdate(carUpdate acserver.Car) error {
	r.UDPCallback(udp.CarUpdate{
		CarID:               carUpdate.CarID,
		Pos:                 carUpdate.PluginStatus.Position,
		Velocity:            carUpdate.PluginStatus.Velocity,
		Gear:                carUpdate.PluginStatus.GearIndex,
		EngineRPM:           carUpdate.PluginStatus.EngineRPM,
		NormalisedSplinePos: carUpdate.PluginStatus.NormalisedSplinePos,
	})

	return nil
}

func (r *UDPPluginAdapter) OnClientEvent(event acserver.ClientEvent) error {
	return nil
}

func (r *UDPPluginAdapter) OnCollisionWithCar(event acserver.ClientEvent) error {
	car, err := r.server.GetCarInfo(event.CarID)

	if err != nil {
		return err
	}

	otherCar, err := r.server.GetCarInfo(event.OtherCarID)

	if err != nil {
		return err
	}

	r.UDPCallback(udp.CollisionWithCar{
		CarID:            event.CarID,
		OtherCarID:       event.OtherCarID,
		ImpactSpeed:      event.Speed,
		WorldPos:         event.Position,
		RelPos:           event.RelativePosition,
		DamageZones:      car.DamageZones,
		OtherDamageZones: otherCar.DamageZones,
	})

	return nil
}

func (r *UDPPluginAdapter) OnCollisionWithEnv(event acserver.ClientEvent) error {
	car, err := r.server.GetCarInfo(event.CarID)

	if err != nil {
		return err
	}

	r.UDPCallback(udp.CollisionWithEnvironment{
		CarID:       event.CarID,
		ImpactSpeed: event.Speed,
		WorldPos:    event.Position,
		RelPos:      event.RelativePosition,
		DamageZones: car.DamageZones,
	})

	return nil
}

func (r *UDPPluginAdapter) OnChat(chat acserver.Chat) error {
	car, err := r.server.GetCarInfo(chat.FromCar)

	if err != nil {
		return err
	}

	r.UDPCallback(udp.Chat{
		CarID:      chat.FromCar,
		Message:    chat.Message,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		DriverName: car.Driver.Name,
		Time:       chat.Time,
	})

	return nil
}

func (r *UDPPluginAdapter) OnConnectionClosed(car acserver.Car) error {
	r.UDPCallback(udp.SessionCarInfo{
		CarID:      car.CarID,
		DriverName: car.Driver.Name,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		CarModel:   car.Model,
		CarSkin:    car.Skin,
		EventType:  udp.EventConnectionClosed,
	})

	return nil
}
