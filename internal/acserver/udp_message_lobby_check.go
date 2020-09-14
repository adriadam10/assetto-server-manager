package acserver

import (
	"net"
)

type LobbyCheckMessageHandler struct {
	state *ServerState
}

func NewConnectionInitialisedMessageHandler(state *ServerState) *LobbyCheckMessageHandler {
	return &LobbyCheckMessageHandler{state: state}
}

func (i LobbyCheckMessageHandler) OnMessage(conn net.PacketConn, addr net.Addr, p *Packet) error {
	w := NewPacket(nil)
	w.Write(UDPMessageConnectionInitialised)
	w.Write(i.state.serverConfig.HTTPPort)

	return w.WriteUDP(conn, addr)
}

func (i LobbyCheckMessageHandler) MessageType() MessageType {
	return UDPMessageConnectionInitialised
}
