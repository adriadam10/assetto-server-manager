package acserver

import (
	"net"
)

type ClientEventMessageHandler struct {
	state  *ServerState
	plugin Plugin
	logger Logger
}

func NewClientEventMessageHandler(state *ServerState, plugin Plugin, logger Logger) *ClientEventMessageHandler {
	return &ClientEventMessageHandler{
		state:  state,
		plugin: plugin,
		logger: logger,
	}
}

type ClientEvent struct {
	CarID            CarID
	DriverGUID       string
	OtherDriverGUID  string
	OtherCarID       CarID
	EventType        MessageType
	Speed            float32
	Position         Vector3F
	RelativePosition Vector3F
}

const (
	EventTypeOtherCar    MessageType = 0xA
	EventTypeEnvironment MessageType = 0xB
)

func (d ClientEventMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	entrant, err := d.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	numEvents := p.ReadUint16()

	for i := 0; i < int(numEvents); i++ {
		clientEvent := &ClientEvent{
			CarID:      entrant.CarID,
			DriverGUID: entrant.Driver.GUID,
		}

		p.Read(&clientEvent.EventType)

		switch clientEvent.EventType {
		case EventTypeOtherCar:
			p.Read(&clientEvent.OtherCarID)
			p.Read(&clientEvent.Speed)
			p.Read(&clientEvent.Position)
			p.Read(&clientEvent.RelativePosition)

			otherCar, err := d.state.GetCarByID(clientEvent.OtherCarID)

			if err != nil {
				return err
			}

			clientEvent.OtherDriverGUID = otherCar.Driver.GUID

			d.logger.Debugf(
				"%s collided with %s at %.2fkm/h (pos: %.2f, %.2f, %.2f) (rel: %.2f, %.2f, %.2f)",
				entrant.Driver.Name,
				otherCar,
				clientEvent.Speed,
				clientEvent.Position.X,
				clientEvent.Position.Y,
				clientEvent.Position.Z,
				clientEvent.RelativePosition.X,
				clientEvent.RelativePosition.Y,
				clientEvent.RelativePosition.Z,
			)

			go func() {
				err := d.plugin.OnCollisionWithCar(*clientEvent)

				if err != nil {
					d.logger.WithError(err).Error("On collision with car plugin returned an error")
				}
			}()
		case EventTypeEnvironment:
			p.Read(&clientEvent.Speed)
			p.Read(&clientEvent.Position)
			p.Read(&clientEvent.RelativePosition)

			d.logger.Debugf(
				"%s collided with environment at %.2fkm/h (pos: %.2f, %.2f, %.2f) (rel: %.2f, %.2f, %.2f)",
				entrant.Driver.Name,
				clientEvent.Speed,
				clientEvent.Position.X,
				clientEvent.Position.Y,
				clientEvent.Position.Z,
				clientEvent.RelativePosition.X,
				clientEvent.RelativePosition.Y,
				clientEvent.RelativePosition.Z,
			)

			go func() {
				err := d.plugin.OnCollisionWithEnv(*clientEvent)

				if err != nil {
					d.logger.WithError(err).Error("On collision with environment plugin returned an error")
				}
			}()
		case 0xC:
			d.logger.Debugf("Client Event type 0xC, what is this? (buf: %x)", p.buf)
		}

		entrant.AddEvent(clientEvent)

		err := d.plugin.OnClientEvent(*clientEvent)

		if err != nil {
			d.logger.WithError(err).Error("On client event plugin returned an error")
		}
	}

	return nil
}

func (d ClientEventMessageHandler) MessageType() MessageType {
	return TCPMessageClientEvent
}
