package acserver

import (
	"crypto/subtle"
	"net"
)

type ChecksumMessageHandler struct {
	state           *ServerState
	checksumManager *ChecksumManager
	logger          Logger
}

func NewChecksumMessageHandler(state *ServerState, checksumManager *ChecksumManager, logger Logger) *ChecksumMessageHandler {
	return &ChecksumMessageHandler{
		state:           state,
		checksumManager: checksumManager,
		logger:          logger,
	}
}

func (c ChecksumMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	var checksum [16]byte
	car, err := c.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	car.SetHasFailedChecksum(false)
	checksumFiles := c.checksumManager.GetFiles()

	for _, file := range checksumFiles {
		p.Read(&checksum)

		if len(file.MD5) == 0 {
			// if no checksum is set we just check that the file exists
			continue
		}

		c.logger.Debugf("Comparing %x with %x for %s", file.MD5, checksum[:], file.Filename)

		if subtle.ConstantTimeCompare(file.MD5, checksum[:]) != 1 {
			c.logger.Infof("Car: %d failed checksum on file '%s'. Kicking from server.", car.CarID, file.Filename)

			car.SetHasFailedChecksum(true)

			if car.HasSentFirstUpdate() {
				err := c.state.Kick(car.CarID, KickReasonChecksumFailed)

				if err != nil {
					return err
				}
			}
			return nil
		}
	}

	c.logger.Debugf("Car: %d passed checksum for %d files", car.CarID, len(checksumFiles))

	return nil
}

func (c ChecksumMessageHandler) MessageType() MessageType {
	return TCPMessageChecksum
}
