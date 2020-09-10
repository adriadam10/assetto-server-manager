package acserver

import (
	"net"
)

type MandatoryPitMessageHandler struct {
	state  *ServerState
	logger Logger
}

func NewMandatoryPitMessageHandler(state *ServerState, logger Logger) *MandatoryPitMessageHandler {
	return &MandatoryPitMessageHandler{
		state:  state,
		logger: logger,
	}
}

func (m MandatoryPitMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	var mandatoryPitCompleted uint8

	p.Read(&mandatoryPitCompleted)

	car, err := m.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	car.SessionData.MandatoryPit = mandatoryPitCompleted != 0

	if car.SessionData.MandatoryPit {
		m.logger.Infof("Car: %s has completed mandatory pit", car)
	} else {
		m.logger.Infof("Car: %s has not completed mandatory pit", car)
	}

	x := NewPacket(nil)
	x.Write(car.CarID)
	x.Write(mandatoryPitCompleted)

	m.state.BroadcastAllTCP(x)

	return nil
}

func (m MandatoryPitMessageHandler) MessageType() MessageType {
	return TCPMandatoryPitCompleted
}
