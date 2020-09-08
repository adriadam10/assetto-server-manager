package acServer

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

const numberOfPingsForAverage = 50

func (pmh PingMessageHandler) OnMessage(conn net.PacketConn, addr net.Addr, p *Packet) error {
	car := pmh.state.GetCarByUDPAddress(addr)

	if car == nil {
		return nil
	}

	time := currentTimeMillisecond()
	theirTime := p.ReadUint32()
	timeOffset := p.ReadUint32()

	if car.Connection.CurrentPingIndex >= numberOfPingsForAverage {
		car.Connection.CurrentPingIndex = 0
	}

	car.Connection.TargetTimeOffset = uint32(time) - timeOffset
	car.Connection.PingCache[car.Connection.CurrentPingIndex] = int32(uint32(time) - theirTime)
	car.Connection.CurrentPingIndex++

	pingSum := int32(0)
	numPings := int32(0)

	for _, ping := range car.Connection.PingCache {
		if ping > 0 {
			pingSum += ping
			numPings++
		}
	}

	if numPings <= 0 || pingSum <= 0 {
		car.Connection.Ping = 0
	} else {
		car.Connection.Ping = pingSum / numPings
	}

	return nil
}

func (pmh PingMessageHandler) MessageType() MessageType {
	return UDPMessagePing
}
