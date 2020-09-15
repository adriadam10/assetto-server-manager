package acserver

import (
	"context"
	"fmt"
	"net"
)

type UDP struct {
	port   uint16
	logger Logger

	messageHandlers map[MessageType]UDPMessageHandler

	packetConn *net.UDPConn
}

func NewUDP(port uint16, server *Server) *UDP {
	u := &UDP{
		port:            port,
		messageHandlers: make(map[MessageType]UDPMessageHandler),
		logger:          server.logger,
	}

	u.initMessageHandlers(server)

	return u
}

func (u *UDP) initMessageHandlers(server *Server) {
	messageHandlers := []UDPMessageHandler{
		NewConnectionInitialisedMessageHandler(server.state),
		NewPositionMessageHandler(server.state, server.weatherManager, server.plugin, server.logger),
		NewAssociateMessageHandler(server.state),
		NewPingMessageHandler(server.state),
		NewSessionInfoHandler(server.state),
	}

	for _, handler := range messageHandlers {
		u.messageHandlers[handler.MessageType()] = handler
	}
}

func (u *UDP) Listen(ctx context.Context) error {
	u.logger.Infof("UDP server listening on port: %d", u.port)

	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", u.port))

	if err != nil {
		return err
	}

	u.packetConn, err = net.ListenUDP("udp", addr)

	if err != nil {
		return err
	}

	go func() {
		for {
			buf := make([]byte, 1024)

			n, addr, err := u.packetConn.ReadFrom(buf)

			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					u.logger.WithError(err).Error("could not read from udp buffer")
					continue
				}
			}

			if err := u.handleConnection(addr, buf[:n]); err != nil {
				u.logger.WithError(err).Error("could not handle udp connection")
				continue
			}
		}
	}()

	<-ctx.Done()
	u.logger.Infof("Closing UDP server")
	return u.packetConn.Close()
}

func (u *UDP) handleConnection(addr net.Addr, b []byte) error {
	p := NewPacket(b)

	var messageType MessageType

	p.Read(&messageType)

	if messageHandler, ok := u.messageHandlers[messageType]; ok {
		err := messageHandler.OnMessage(u.packetConn, addr, p)

		if err != nil {
			return err
		}
	} else {
		u.logger.Errorf("Unknown UDP message: 0x%x %d (len: %d)", messageType, messageType, len(b))
	}

	return nil
}

func (u *UDP) WriteTo(b []byte, addr net.Addr) (int, error) {
	return u.packetConn.WriteTo(b, addr)
}
