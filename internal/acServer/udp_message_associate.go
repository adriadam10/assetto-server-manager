package acServer

import (
	"net"
)

type AssociateMessageHandler struct {
	state *ServerState
}

func NewAssociateMessageHandler(state *ServerState) *AssociateMessageHandler {
	return &AssociateMessageHandler{state: state}
}

func (a AssociateMessageHandler) OnMessage(conn net.PacketConn, addr net.Addr, p *Packet) error {
	carID := p.ReadCarID()

	if err := a.state.AssociateUDPConnectionByCarID(addr, carID); err != nil {
		return err
	}

	w := NewPacket(nil)
	w.Write(UDPMessageAssociate)

	return w.WriteUDP(conn, addr)
}

func (a AssociateMessageHandler) MessageType() MessageType {
	return UDPMessageAssociate
}
