package acServer

import (
	"net"
)

type TyreChangeMessageHandler struct {
	state *ServerState
}

func NewTyreChangeMessageHandler(state *ServerState) *TyreChangeMessageHandler {
	return &TyreChangeMessageHandler{state: state}
}

func (t TyreChangeMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	tyre := p.ReadString()

	entrant, err := t.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	return t.state.ChangeTyre(entrant.CarID, tyre)
}

func (t TyreChangeMessageHandler) MessageType() MessageType {
	return TCPMessageTyreChange
}
