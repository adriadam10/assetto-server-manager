package acserver

import (
	"net"
	"time"
)

type PositionMessageHandler struct {
	state          *ServerState
	weatherManager *WeatherManager
	plugin         Plugin
	logger         Logger
}

func NewPositionMessageHandler(state *ServerState, weatherManager *WeatherManager, plugin Plugin, logger Logger) *PositionMessageHandler {
	ph := &PositionMessageHandler{
		state:          state,
		weatherManager: weatherManager,
		plugin:         plugin,
		logger:         logger,
	}

	return ph
}

const (
	forceHeadlightByte = 0b100000
)

type CarUpdate struct {
	Sequence            uint8
	Timestamp           uint32
	Position            Vector3F
	Rotation            Vector3F
	Velocity            Vector3F
	TyreAngularSpeed    [4]uint8
	SteerAngle          uint8
	WheelAngle          uint8
	EngineRPM           uint16
	GearIndex           uint8
	StatusBytes         uint32
	PerformanceDelta    int16
	Gas                 uint8
	NormalisedSplinePos float32
}

func (pm *PositionMessageHandler) OnMessage(_ net.PacketConn, addr net.Addr, p *Packet) error {
	var carUpdate CarUpdate

	p.Read(&carUpdate)

	car := pm.state.GetCarByUDPAddress(addr)

	if car == nil {
		return nil
	}

	if pm.state.raceConfig.ForceOpponentHeadlights {
		carUpdate.StatusBytes |= forceHeadlightByte
	}

	if car.HasSentFirstUpdate() && carUpdate.Timestamp < car.PluginStatus.Timestamp {
		pm.logger.Warnf("Position packet out of order for %s previous: %d received: %d", car.Driver.Name, car.PluginStatus.Timestamp, carUpdate.Timestamp)

		return nil
	}

	if !car.HasSentFirstUpdate() || (pm.state.currentSession.SessionType != SessionTypeQualifying || (pm.state.currentSession.SessionType == SessionTypeQualifying && !pm.state.currentSession.Solo)) {
		car.mutex.Lock()
		car.Status = carUpdate

		if pm.state.currentSession.SessionType == SessionTypeQualifying && pm.state.currentSession.Solo {
			car.Status.Velocity = Vector3F{
				X: 0,
				Y: 0,
				Z: 0,
			}
		}
		car.mutex.Unlock()
	}

	car.SetHasUpdateToBroadcast(true)

	car.mutex.Lock()
	{
		car.Status.Timestamp += car.Connection.TimeOffset
		car.PluginStatus = carUpdate

		diff := int(car.Connection.TargetTimeOffset) - int(car.Connection.TimeOffset)

		var v13, v14 int

		if diff >= 0 {
			v13 = diff
			v14 = diff
		} else {
			v14 = int(car.Connection.TimeOffset) - int(car.Connection.TargetTimeOffset)
		}

		if v13 > 0 || v13 == 0 && v14 > 1000 {
			car.Connection.TimeOffset = car.Connection.TargetTimeOffset
		} else if v13 == 0 && v14 < 3 || v13 < 0 {
			car.Connection.TimeOffset = car.Connection.TargetTimeOffset
		} else {
			if diff > 0 {
				car.Connection.TimeOffset = car.Connection.TimeOffset + 3
			}

			if diff < 0 {
				car.Connection.TimeOffset = car.Connection.TimeOffset - 3
			}
		}
	}
	car.mutex.Unlock()

	if !car.HasSentFirstUpdate() {
		if car.Connection.FailedChecksum {
			if err := pm.state.Kick(car.CarID, KickReasonChecksumFailed); err != nil {
				return err
			}
		}

		car.SetHasSentFirstUpdate(true)

		if err := pm.SendFirstUpdate(car); err != nil {
			return err
		}

		err := pm.plugin.OnClientLoaded(car.Copy())

		if err != nil {
			pm.logger.WithError(err).Error("On client loaded plugin returned an error")
		}
	}

	return nil
}

func (pm *PositionMessageHandler) SendFirstUpdate(car *Car) error {
	pm.logger.Infof("Sending first update to client: %s", car.String())

	bw := NewPacket(nil)
	bw.Write(TCPConnectedEntrants)
	bw.Write(uint8(len(pm.state.entryList)))

	for _, entrant := range pm.state.entryList {
		bw.Write(entrant.CarID)
		bw.WriteUTF32String(entrant.Driver.Name)
	}

	if err := bw.WriteTCP(car.Connection.tcpConn); err != nil {
		return err
	}

	// send weather to car
	if err := pm.weatherManager.SendWeather(car); err != nil {
		return err
	}

	// send a lap completed message for car ID 0xFF to broadcast all other lap times to the connecting user.
	if err := pm.state.CompleteLap(ServerCarID, &LapCompleted{}, car); err != nil {
		return err
	}

	for _, otherEntrant := range pm.state.entryList {
		if car.CarID == otherEntrant.CarID {
			continue
		}

		bw := NewPacket(nil)
		bw.Write(TCPMessageTyreChange)
		bw.Write(otherEntrant.CarID)
		bw.WriteString(otherEntrant.Tyres)

		if err := bw.WriteTCP(car.Connection.tcpConn); err != nil {
			return err
		}

		bw = NewPacket(nil)

		bw.Write(TCPMessageCarID)
		bw.Write(otherEntrant.CarID)
		bw.Write(otherEntrant.SessionData.P2PCount)
		bw.Write(uint8(0))

		if err := bw.WriteTCP(car.Connection.tcpConn); err != nil {
			return err
		}

		bw = NewPacket(nil)
		bw.Write(TCPMandatoryPitCompleted)
		bw.Write(otherEntrant.CarID)

		if otherEntrant.SessionData.MandatoryPit {
			bw.Write(uint8(0x01))
		} else {
			bw.Write(uint8(0x00))
		}

		if err := bw.WriteTCP(car.Connection.tcpConn); err != nil {
			return err
		}
	}

	car.mutex.Lock()
	car.Driver.LoadTime = time.Now()
	car.mutex.Unlock()

	// send bop for car
	if err := pm.state.SendBoP(car); err != nil {
		return err
	}

	// send MOTD to the newly connected car
	if err := pm.state.SendMOTD(car); err != nil {
		return err
	}

	// send fixed setup too
	if err := pm.state.SendSetup(car); err != nil {
		return err
	}

	// if there are drs zones, send them too
	if err := pm.state.SendDRSZones(car); err != nil {
		return err
	}

	if err := pm.weatherManager.SendSunAngleToCar(car); err != nil {
		return err
	}

	return nil
}

func (pm *PositionMessageHandler) MessageType() MessageType {
	return UDPMessageCarUpdate
}
