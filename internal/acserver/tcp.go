package acserver

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

type TCP struct {
	port uint16

	messageHandlers map[MessageType]TCPMessageHandler

	listener *net.TCPListener
	state    *ServerState
	logger   Logger
}

func NewTCP(port uint16, server *Server) *TCP {
	tcp := &TCP{
		port:            port,
		messageHandlers: make(map[MessageType]TCPMessageHandler),
		state:           server.state,
		logger:          server.logger,
	}

	tcp.initMessageHandlers(server)

	return tcp
}

func (t *TCP) initMessageHandlers(server *Server) {
	votingManager := NewVotingManager(server.state, server.sessionManager, server.logger)

	messageHandlers := []TCPMessageHandler{
		NewHandshakeMessageHandler(server.state, server.sessionManager, server.entryListManager, server.weatherManager, server.checksumManager, server.dynamicTrack, server.plugin, server.logger),
		NewEntryListMessageHandler(server.state, server.logger),
		NewCarIDMessageHandler(server.state, server.logger),
		NewChecksumMessageHandler(server.state, server.checksumManager, server.logger),
		NewTyreChangeMessageHandler(server.state),
		NewLapCompletedMessageHandler(server.state, server.sessionManager),
		NewSectorSplitMessageHandler(server.state, server.plugin, server.logger),
		NewBroadcastChatMessageHandler(server.state, server.sessionManager, server.adminCommandManager),
		NewDamageZonesMessageHandler(server.state, server.sessionManager, server.logger),
		NewClientEventMessageHandler(server.state, server.plugin, server.logger),
		NewDisconnectMessageHandler(server.state),
		NewMandatoryPitMessageHandler(server.state, server.logger),
		NewVoteNextSessionHandler(votingManager),
		NewVoteRestartSessionHandler(votingManager),
		NewVoteKickHandler(votingManager),
	}

	for _, handler := range messageHandlers {
		t.messageHandlers[handler.MessageType()] = handler
	}
}

// tcpConn wraps a net.Conn and provides a clean closer to shut down active listeners.
type tcpConn struct {
	net.Conn
	closer chan struct{}
}

func (t *TCP) Listen(ctx context.Context) error {
	t.logger.Infof("TCP server listening on port: %d", t.port)

	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf(":%d", t.port))

	if err != nil {
		return err
	}

	t.listener, err = net.ListenTCP("tcp", addr)

	if err != nil {
		return err
	}

	go func() {
		for {
			conn, err := t.listener.AcceptTCP()

			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					t.logger.WithError(err).Error("couldn't accept tcp connection")
					continue
				}
			}

			if err := conn.SetKeepAlive(true); err != nil {
				t.logger.WithError(err).Error("Could not set keepalive on connection")
			}

			if err := conn.SetKeepAlivePeriod(time.Minute); err != nil {
				t.logger.WithError(err).Error("Could not set keepalive timeout on connection")
			}

			if err := conn.SetNoDelay(true); err != nil {
				t.logger.WithError(err).Error("Could not set no delay on connection")
			}

			c := &tcpConn{
				Conn:   conn,
				closer: make(chan struct{}, 1),
			}

			go func(conn *tcpConn) {
				defer conn.Close()

				for {
					select {
					case <-ctx.Done():
						return
					case <-conn.closer:
						car, _ := t.state.GetCarByTCPConn(conn)

						if err := conn.Close(); err != nil {
							t.logger.WithError(err).Errorf("Could not close tcp connection for: %s", conn.RemoteAddr().String())
						} else {
							t.logger.Debugf("Cleanly closed tcp connection for: %s", conn.RemoteAddr().String())
						}

						if car != nil {
							car.CloseConnection()
						}

						return
					default:
						var messageLength uint16

						if err := binary.Read(conn, binary.LittleEndian, &messageLength); err != nil {
							if e, ok := err.(*net.OpError); ok && (!e.Temporary() || e.Timeout()) {
								t.logger.WithError(err).Errorf("Detected broken TCP connection for: %s. Closing now.", conn.RemoteAddr().String())
								t.state.closeTCPConnection(conn)
								continue
							}

							t.logger.WithError(err).Error("couldn't handle tcp connection (read message length)")
							return
						}

						if err = t.handleConnection(conn, messageLength); err != nil {
							if e, ok := err.(*net.OpError); ok && (!e.Temporary() || e.Timeout()) {
								t.logger.WithError(err).Errorf("Detected broken TCP connection for: %s. Closing now.", conn.RemoteAddr().String())
								t.state.closeTCPConnection(conn)
								continue
							}

							t.logger.WithError(err).Error("couldn't handle tcp connection (read message)")
							return
						}
					}
				}
			}(c)
		}
	}()

	<-ctx.Done()
	t.logger.Infof("Closing TCP server")
	return t.listener.Close()
}

func (t *TCP) handleConnection(conn net.Conn, messageLength uint16) error {
	buf := make([]byte, messageLength)

	n, err := conn.Read(buf)

	if err != nil {
		return err
	}

	var messageType MessageType

	p := NewPacket(buf[:n])
	p.Read(&messageType)

	messageHandler, ok := t.messageHandlers[messageType]

	if ok {
		if err := messageHandler.OnMessage(conn, p); err != nil {
			t.logger.WithError(err).Errorf("Message Handler: 0x%x returned error", messageHandler.MessageType())
			return err
		}
	} else {
		t.logger.Errorf("Unknown TCP message type: 0x%x (len: %d)", messageType, n)
		t.logger.Printf("%x", buf[:n])
	}

	return nil
}
