package acServer

import (
	"net"
)

type DamageZonesMessageHandler struct {
	state  *ServerState
	logger Logger
}

func NewDamageZonesMessageHandler(state *ServerState, logger Logger) *DamageZonesMessageHandler {
	return &DamageZonesMessageHandler{
		state:  state,
		logger: logger,
	}
}

func (d DamageZonesMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	entrant, err := d.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	p.Read(&entrant.DamageZones)

	d.logger.Debugf(
		"%s reported damage: front bumper %.3f, rear bumper %.3f, left skirt %.3f, right skirt %.3f, unknown %.3f",
		entrant.Driver.Name,
		entrant.DamageZones[0],
		entrant.DamageZones[1],
		entrant.DamageZones[2],
		entrant.DamageZones[3],
		entrant.DamageZones[4],
	)

	return d.state.BroadcastDamageZones(entrant)
}

func (d DamageZonesMessageHandler) MessageType() MessageType {
	return TCPMessageDamageZones
}
