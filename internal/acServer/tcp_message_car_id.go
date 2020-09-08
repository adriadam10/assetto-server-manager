package acServer

import (
	"net"
)

type CarIDMessageHandler struct {
	state  *ServerState
	logger Logger
}

func NewCarIDMessageHandler(state *ServerState, logger Logger) *CarIDMessageHandler {
	return &CarIDMessageHandler{
		state:  state,
		logger: logger,
	}
}

func (c CarIDMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	entrant, err := c.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	c.logger.Debugf("Received CarID message")

	p2pCount := p.ReadInt16()

	if p2pCount == -1 {
		o := NewPacket(nil)
		o.Write(TCPMessageCarID)
		o.Write(entrant.CarID)
		o.Write(entrant.SessionData.P2PCount)
		o.Write(uint8(0))

		return o.WriteTCP(conn)
	} else {
		c.logger.Debugf("P2P Count what is the number: %d", p2pCount)
		entrant.SessionData.P2PCount = p2pCount

		o := NewPacket(nil)
		o.Write(TCPMessageCarID)
		o.Write(entrant.CarID)
		o.Write(entrant.SessionData.P2PCount)        // @TODO does this ever change?
		o.Write(uint8(entrant.SessionData.P2PCount)) // bool, always seems to be 0

		c.state.BroadcastAllTCP(o)
	}

	return nil
}

func (c CarIDMessageHandler) MessageType() MessageType {
	return TCPMessageCarID
}
