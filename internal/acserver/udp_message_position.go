package acserver

import (
	"net"
	"time"
)

type PositionMessageHandler struct {
	state  *ServerState
	plugin Plugin
	logger Logger
}

func NewPositionMessageHandler(state *ServerState, plugin Plugin, logger Logger) *PositionMessageHandler {
	ph := &PositionMessageHandler{
		state:  state,
		plugin: plugin,
		logger: logger,
	}

	return ph
}

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

func (pm *PositionMessageHandler) OnMessage(conn net.PacketConn, addr net.Addr, p *Packet) error {
	var carUpdate CarUpdate

	p.Read(&carUpdate)

	entrant := pm.state.GetCarByUDPAddress(addr)

	if entrant == nil {
		return nil
	}

	if !entrant.Connection.HasSentFirstUpdate || (pm.state.currentSession.SessionType != SessionTypeQualifying || (pm.state.currentSession.SessionType == SessionTypeQualifying && !pm.state.currentSession.Solo)) {
		entrant.Status = carUpdate

		if pm.state.currentSession.SessionType == SessionTypeQualifying && pm.state.currentSession.Solo {
			entrant.Status.Velocity = Vector3F{
				X: 0,
				Y: 0,
				Z: 0,
			}
		}
	}

	entrant.Status.Timestamp += entrant.Connection.TimeOffset
	entrant.HasUpdateToBroadcast = true

	diff := int(entrant.Connection.TargetTimeOffset) - int(entrant.Connection.TimeOffset)

	var v13, v14 int

	if diff >= 0 {
		v13 = diff
		v14 = diff
	} else {
		v14 = int(entrant.Connection.TimeOffset) - int(entrant.Connection.TargetTimeOffset)
	}

	if v13 > 0 || v13 == 0 && v14 > 1000 {
		entrant.Connection.TimeOffset = entrant.Connection.TargetTimeOffset
	} else if v13 == 0 && v14 < 3 || v13 < 0 {
		entrant.Connection.TimeOffset = entrant.Connection.TargetTimeOffset
	} else {
		if diff > 0 {
			entrant.Connection.TimeOffset = entrant.Connection.TimeOffset + 3
		}

		if diff < 0 {
			entrant.Connection.TimeOffset = entrant.Connection.TimeOffset - 3
		}
	}

	if !entrant.Connection.HasSentFirstUpdate {
		if entrant.Connection.FailedChecksum {
			if err := pm.state.Kick(entrant.CarID, KickReasonChecksumFailed); err != nil {
				return err
			}
		}

		entrant.Connection.HasSentFirstUpdate = true

		if err := pm.SendFirstUpdate(entrant); err != nil {
			return err
		}

		go func() {
			err := pm.plugin.OnClientLoaded(*entrant)

			if err != nil {
				pm.logger.WithError(err).Error("On client loaded plugin returned an error")
			}
		}()
	}

	go func() {
		err := pm.plugin.OnCarUpdate(*entrant)

		if err != nil {
			pm.logger.WithError(err).Error("On car update plugin returned an error")
		}
	}()

	return nil
}

func (pm *PositionMessageHandler) SendFirstUpdate(entrant *Car) error {
	pm.logger.Infof("Sending first update to client: %s", entrant.String())

	bw := NewPacket(nil)
	bw.Write(TCPConnectedEntrants)
	bw.Write(uint8(len(pm.state.entryList)))

	for _, entrant := range pm.state.entryList {
		bw.Write(entrant.CarID)
		bw.WriteUTF32String(entrant.Driver.Name)
	}

	if err := bw.WriteTCP(entrant.Connection.tcpConn); err != nil {
		return err
	}

	// send weather to car
	if err := pm.state.SendWeather(entrant); err != nil {
		return err
	}

	// send a lap completed message for car ID 0xFF to broadcast all other lap times to the connecting user.
	if err := pm.state.CompleteLap(ServerCarID, &LapCompleted{}, entrant); err != nil {
		return err
	}

	for _, otherEntrant := range pm.state.entryList {
		if entrant.CarID == otherEntrant.CarID {
			continue
		}

		bw := NewPacket(nil)
		bw.Write(TCPMessageTyreChange)
		bw.Write(otherEntrant.CarID)
		bw.WriteString(otherEntrant.Tyres)

		if err := bw.WriteTCP(entrant.Connection.tcpConn); err != nil {
			return err
		}

		bw = NewPacket(nil)

		bw.Write(TCPMessageCarID)
		bw.Write(otherEntrant.CarID)
		bw.Write(otherEntrant.SessionData.P2PCount)
		bw.Write(uint8(0))

		if err := bw.WriteTCP(entrant.Connection.tcpConn); err != nil {
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

		if err := bw.WriteTCP(entrant.Connection.tcpConn); err != nil {
			return err
		}

		entrant.Driver.LoadTime = time.Now()
	}

	// send bop for car
	if err := pm.state.SendBoP(entrant); err != nil {
		return err
	}

	// send MOTD to the newly connected car
	if err := pm.state.SendMOTD(entrant); err != nil {
		return err
	}

	// send fixed setup too
	if err := pm.state.SendSetup(entrant); err != nil {
		return err
	}

	// if there are drs zones, send them too
	if err := pm.state.SendDRSZones(entrant); err != nil {
		return err
	}

	return nil
}

func (pm *PositionMessageHandler) MessageType() MessageType {
	return UDPMessageCarUpdate
}
