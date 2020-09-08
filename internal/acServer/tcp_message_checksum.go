package acServer

import (
	"crypto/subtle"
	"net"
)

type ChecksumMessageHandler struct {
	state  *ServerState
	logger Logger
}

func NewChecksumMessageHandler(state *ServerState, logger Logger) *ChecksumMessageHandler {
	return &ChecksumMessageHandler{
		state:  state,
		logger: logger,
	}
}

func (c ChecksumMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	var checksum [16]byte
	entrant, err := c.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	entrant.Connection.FailedChecksum = false

	for _, file := range c.state.checkSummableFiles {
		p.Read(&checksum)

		if len(file.MD5) == 0 {
			// if no checksum is set we just check that the file exists
			continue
		}

		c.logger.Debugf("Comparing %x with %x for %s", file.MD5, checksum[:], file.Filename)

		if subtle.ConstantTimeCompare(file.MD5, checksum[:]) != 1 {
			c.logger.Infof("Car: %d failed checksum on file '%s'. Kicking from server.", entrant.CarID, file.Filename)

			entrant.Connection.FailedChecksum = true
			return nil
		}
	}

	c.logger.Debugf("Car: %d passed checksum for %d files", entrant.CarID, len(c.state.checkSummableFiles))

	return nil
}

func (c ChecksumMessageHandler) MessageType() MessageType {
	return TCPMessageChecksum
}
