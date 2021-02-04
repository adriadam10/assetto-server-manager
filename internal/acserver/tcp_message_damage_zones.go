package acserver

import (
	"net"
)

type DamageZonesMessageHandler struct {
	state          *ServerState
	sessionManager *SessionManager
	logger         Logger
}

func NewDamageZonesMessageHandler(state *ServerState, sessionManager *SessionManager, logger Logger) *DamageZonesMessageHandler {
	return &DamageZonesMessageHandler{
		state:          state,
		sessionManager: sessionManager,
		logger:         logger,
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

	return d.BroadcastDamageZones(entrant)
}

func (d DamageZonesMessageHandler) BroadcastDamageZones(entrant *Car) error {
	currentSession := d.sessionManager.GetCurrentSession()

	if currentSession.SessionType == SessionTypeQualifying && currentSession.Solo {
		return nil
	}

	p := NewPacket(nil)

	p.Write(TCPMessageDamageZones)
	p.Write(entrant.CarID)
	p.Write(entrant.DamageZones)

	d.state.BroadcastOthersTCP(p, entrant.CarID)

	return nil
}

func (d DamageZonesMessageHandler) MessageType() MessageType {
	return TCPMessageDamageZones
}
