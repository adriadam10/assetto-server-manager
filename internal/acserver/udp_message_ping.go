package acserver

import (
	"net"
)

type PingMessageHandler struct {
	state *ServerState
}

func NewPingMessageHandler(state *ServerState) *PingMessageHandler {
	return &PingMessageHandler{
		state: state,
	}
}

func (pmh PingMessageHandler) OnMessage(conn net.PacketConn, addr net.Addr, p *Packet) error {
	car := pmh.state.GetCarByUDPAddress(addr)

	if car == nil {
		return nil
	}

	time := pmh.state.CurrentTimeMillisecond()
	theirTime := p.ReadUint32()
	timeOffset := p.ReadUint32()

	car.UpdatePing(time, theirTime, timeOffset)

	return nil
}

func (pmh PingMessageHandler) MessageType() MessageType {
	return UDPMessagePing
}
