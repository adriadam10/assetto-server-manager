package acsm

import (
	"justapengu.in/acsm/internal/acServer"
	"justapengu.in/acsm/pkg/udp"
)

type RaceControlAdapter struct {
	server      *acServer.Server
	raceControl *RaceControl
}

func NewRaceControlAdapter(raceControl *RaceControl) acServer.Plugin {
	return &RaceControlAdapter{
		raceControl: raceControl,
	}
}

func (r *RaceControlAdapter) Init(server *acServer.Server, _ acServer.Logger) error {
	r.server = server
	r.raceControl.server = server

	return nil
}

func (r *RaceControlAdapter) OnVersion(version uint16) error {
	r.raceControl.UDPCallback(udp.Version(version))

	return nil
}

func convertSessionInfoToUDP(eventType udp.Event, session acServer.SessionInfo) udp.SessionInfo {
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
		EventType:           eventType,
	}
}

func (r *RaceControlAdapter) OnNewSession(newSession acServer.SessionInfo) error {
	r.raceControl.UDPCallback(convertSessionInfoToUDP(udp.EventNewSession, newSession))

	return nil
}

func (r *RaceControlAdapter) OnWeatherChange(_ acServer.CurrentWeather) error {
	r.raceControl.UDPCallback(convertSessionInfoToUDP(udp.EventSessionInfo, r.server.GetSessionInfo()))

	return nil
}

func (r *RaceControlAdapter) OnEndSession(sessionFile string) error {
	r.raceControl.UDPCallback(udp.EndSession(sessionFile))

	return nil
}

func (r *RaceControlAdapter) OnNewConnection(car acServer.Car) error {
	r.raceControl.UDPCallback(udp.SessionCarInfo{
		CarID:      car.CarID,
		DriverName: car.Driver.Name,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		CarModel:   car.Model,
		CarSkin:    car.Skin,
		EventType:  udp.EventNewConnection,
	})

	return nil
}

func (r *RaceControlAdapter) OnClientLoaded(car acServer.Car) error {
	r.raceControl.UDPCallback(udp.ClientLoaded(car.CarID))

	return nil
}

func (r *RaceControlAdapter) OnSectorCompleted(split acServer.Split) error {
	return nil
}

func (r *RaceControlAdapter) OnLapCompleted(carID acServer.CarID, lap acServer.Lap) error {
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

	r.raceControl.UDPCallback(l)

	return nil
}

func (r *RaceControlAdapter) OnCarUpdate(carUpdate acServer.Car) error {
	r.raceControl.UDPCallback(udp.CarUpdate{
		CarID:               carUpdate.CarID,
		Pos:                 carUpdate.Status.Position,
		Velocity:            carUpdate.Status.Velocity,
		Gear:                carUpdate.Status.GearIndex,
		EngineRPM:           carUpdate.Status.EngineRPM,
		NormalisedSplinePos: carUpdate.Status.NormalisedSplinePos,
	})

	return nil
}

func (r *RaceControlAdapter) OnClientEvent(event acServer.ClientEvent) error {
	return nil
}

func (r *RaceControlAdapter) OnCollisionWithCar(event acServer.ClientEvent) error {
	r.raceControl.UDPCallback(udp.CollisionWithCar{
		CarID:       event.CarID,
		OtherCarID:  event.OtherCarID,
		ImpactSpeed: event.Speed,
		WorldPos:    event.Position,
		RelPos:      event.RelativePosition,
	})

	return nil
}

func (r *RaceControlAdapter) OnCollisionWithEnv(event acServer.ClientEvent) error {
	r.raceControl.UDPCallback(udp.CollisionWithEnvironment{
		CarID:       event.CarID,
		ImpactSpeed: event.Speed,
		WorldPos:    event.Position,
		RelPos:      event.RelativePosition,
	})

	return nil
}

func (r *RaceControlAdapter) OnChat(chat acServer.Chat) error {
	car, err := r.server.GetCarInfo(chat.FromCar)

	if err != nil {
		return err
	}

	r.raceControl.UDPCallback(udp.Chat{
		CarID:      chat.FromCar,
		Message:    chat.Message,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		DriverName: car.Driver.Name,
		Time:       chat.Time,
	})

	return nil
}

func (r *RaceControlAdapter) OnConnectionClosed(car acServer.Car) error {
	r.raceControl.UDPCallback(udp.SessionCarInfo{
		CarID:      car.CarID,
		DriverName: car.Driver.Name,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		CarModel:   car.Model,
		CarSkin:    car.Skin,
		EventType:  udp.EventConnectionClosed,
	})

	return nil
}
