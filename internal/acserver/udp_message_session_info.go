package acserver

import (
	"net"
)

type SessionInfoHandler struct {
	state *ServerState
}

func NewSessionInfoHandler(state *ServerState) *SessionInfoHandler {
	return &SessionInfoHandler{state: state}
}

func (s SessionInfoHandler) OnMessage(_ net.PacketConn, addr net.Addr, p *Packet) error {
	entrant := s.state.GetCarByUDPAddress(addr)

	gameThinksWeAreInSessionType := SessionType(p.ReadUint8())

	if gameThinksWeAreInSessionType == s.state.currentSession.SessionType {
		return nil
	}

	return s.state.SendSessionInfo(entrant, nil)
}

func (s SessionInfoHandler) MessageType() MessageType {
	return UDPMessageSessionInfo
}
