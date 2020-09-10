package acsm

import (
	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/udp"
)

type RaceControlAdapter struct {
	*RaceControl
	server acserver.ServerPlugin
}

func NewRaceControlAdapter(raceControl *RaceControl) acserver.Plugin {
	return &RaceControlAdapter{
		RaceControl: raceControl,
	}
}

func (r *RaceControlAdapter) Init(server acserver.ServerPlugin, _ acserver.Logger) error {
	r.server = server
	r.RaceControl.server = server

	return nil
}

func (r *RaceControlAdapter) OnVersion(version uint16) error {
	r.RaceControl.UDPCallback(udp.Version(version))

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
		EventType:           eventType,
	}
}

func (r *RaceControlAdapter) OnNewSession(newSession acserver.SessionInfo) error {
	r.RaceControl.UDPCallback(convertSessionInfoToUDP(udp.EventNewSession, newSession))

	return nil
}

func (r *RaceControlAdapter) OnWeatherChange(_ acserver.CurrentWeather) error {
	r.RaceControl.UDPCallback(convertSessionInfoToUDP(udp.EventSessionInfo, r.server.GetSessionInfo()))

	return nil
}

func (r *RaceControlAdapter) OnEndSession(sessionFile string) error {
	r.RaceControl.UDPCallback(udp.EndSession(sessionFile))

	return nil
}

func (r *RaceControlAdapter) OnNewConnection(car acserver.Car) error {
	r.RaceControl.UDPCallback(udp.SessionCarInfo{
		CarID:      car.CarID,
		DriverName: car.Driver.Name,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		CarModel:   car.Model,
		CarSkin:    car.Skin,
		EventType:  udp.EventNewConnection,
	})

	return nil
}

func (r *RaceControlAdapter) OnClientLoaded(car acserver.Car) error {
	r.RaceControl.UDPCallback(udp.ClientLoaded(car.CarID))

	return nil
}

func (r *RaceControlAdapter) OnSectorCompleted(split acserver.Split) error {
	return nil
}

func (r *RaceControlAdapter) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {
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

	r.RaceControl.UDPCallback(l)

	return nil
}

func (r *RaceControlAdapter) OnCarUpdate(carUpdate acserver.Car) error {
	r.RaceControl.UDPCallback(udp.CarUpdate{
		CarID:               carUpdate.CarID,
		Pos:                 carUpdate.Status.Position,
		Velocity:            carUpdate.Status.Velocity,
		Gear:                carUpdate.Status.GearIndex,
		EngineRPM:           carUpdate.Status.EngineRPM,
		NormalisedSplinePos: carUpdate.Status.NormalisedSplinePos,
	})

	return nil
}

func (r *RaceControlAdapter) OnClientEvent(event acserver.ClientEvent) error {
	return nil
}

func (r *RaceControlAdapter) OnCollisionWithCar(event acserver.ClientEvent) error {
	r.RaceControl.UDPCallback(udp.CollisionWithCar{
		CarID:       event.CarID,
		OtherCarID:  event.OtherCarID,
		ImpactSpeed: event.Speed,
		WorldPos:    event.Position,
		RelPos:      event.RelativePosition,
	})

	return nil
}

func (r *RaceControlAdapter) OnCollisionWithEnv(event acserver.ClientEvent) error {
	r.RaceControl.UDPCallback(udp.CollisionWithEnvironment{
		CarID:       event.CarID,
		ImpactSpeed: event.Speed,
		WorldPos:    event.Position,
		RelPos:      event.RelativePosition,
	})

	return nil
}

func (r *RaceControlAdapter) OnChat(chat acserver.Chat) error {
	car, err := r.server.GetCarInfo(chat.FromCar)

	if err != nil {
		return err
	}

	r.RaceControl.UDPCallback(udp.Chat{
		CarID:      chat.FromCar,
		Message:    chat.Message,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		DriverName: car.Driver.Name,
		Time:       chat.Time,
	})

	return nil
}

func (r *RaceControlAdapter) OnConnectionClosed(car acserver.Car) error {
	r.RaceControl.UDPCallback(udp.SessionCarInfo{
		CarID:      car.CarID,
		DriverName: car.Driver.Name,
		DriverGUID: udp.DriverGUID(car.Driver.GUID),
		CarModel:   car.Model,
		CarSkin:    car.Skin,
		EventType:  udp.EventConnectionClosed,
	})

	return nil
}
