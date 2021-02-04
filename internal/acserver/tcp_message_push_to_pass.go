package acserver

import (
	"net"
)

type PushToPassMessageHandler struct {
	state  *ServerState
	logger Logger
}

func NewPushToPassMessageHandler(state *ServerState, logger Logger) *PushToPassMessageHandler {
	return &PushToPassMessageHandler{
		state:  state,
		logger: logger,
	}
}

func (c PushToPassMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	car, err := c.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	p2pCount := p.ReadUint16()
	isActive := p.ReadUint8()

	if car.SessionData.P2PCount != -1 && p2pCount == 65535 {
		// this message is 'read' the push to pass count on first join
		c.logger.Debugf("Received Push To Pass for CarID: %d. Has %d Push to Pass left (Active: %d)", car.CarID, car.SessionData.P2PCount, isActive)

		o := NewPacket(nil)
		o.Write(TCPMessagePushToPass)
		o.Write(car.CarID)
		o.Write(car.SessionData.P2PCount)
		o.Write(uint8(0))

		c.state.WritePacket(o, conn)
		return nil
	}

	// down here is 'write' push to pass count.

	c.logger.Debugf("Received Push To Pass for CarID: %d. Has %d Push to Pass left (Active: %d)", car.CarID, p2pCount, isActive)
	car.SessionData.P2PCount = int16(p2pCount)

	o := NewPacket(nil)
	o.Write(TCPMessagePushToPass)
	o.Write(car.CarID)
	o.Write(car.SessionData.P2PCount)
	o.Write(isActive)

	c.state.BroadcastAllTCP(o)

	return nil
}

func (c PushToPassMessageHandler) MessageType() MessageType {
	return TCPMessagePushToPass
}
