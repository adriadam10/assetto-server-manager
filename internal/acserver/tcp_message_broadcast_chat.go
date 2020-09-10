package acserver

import (
	"net"
	"strings"
)

type BroadcastChatMessageHandler struct {
	state               *ServerState
	sessionManager      *SessionManager
	adminCommandManager *AdminCommandManager
}

func NewBroadcastChatMessageHandler(state *ServerState, sessionManager *SessionManager, adminCommandManager *AdminCommandManager) *BroadcastChatMessageHandler {
	return &BroadcastChatMessageHandler{
		state:               state,
		sessionManager:      sessionManager,
		adminCommandManager: adminCommandManager,
	}
}

type BroadcastChat struct {
	CarID   CarID
	Message string
}

func (b BroadcastChatMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	chat := &BroadcastChat{}

	p.Read(&chat.CarID)
	chat.Message = p.ReadUTF32String()

	if strings.HasPrefix(chat.Message, "/") {
		entrant, err := b.state.GetCarByTCPConn(conn)

		if err != nil {
			return err
		}

		return b.adminCommandManager.Command(entrant, chat.Message)
	}

	b.state.BroadcastChat(chat.CarID, chat.Message)

	return nil
}

func (b BroadcastChatMessageHandler) MessageType() MessageType {
	return TCPMessageBroadcastChat
}
