package acserver

import (
	"net"
	"time"
)

type SectorSplitMessageHandler struct {
	state  *ServerState
	plugin Plugin
	logger Logger
}

func NewSectorSplitMessageHandler(state *ServerState, plugin Plugin, logger Logger) *SectorSplitMessageHandler {
	return &SectorSplitMessageHandler{
		state:  state,
		plugin: plugin,
		logger: logger,
	}
}

type Split struct {
	Index uint8
	Time  uint32
	Cuts  uint8
}

func (t SectorSplitMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	car, err := t.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	splitIndex := p.ReadUint8()
	splitTime := p.ReadUint32()
	cuts := p.ReadUint8()

	t.logger.Infof("CarID: %d just completed split index: %d with time %s (%d cuts)", car.CarID, splitIndex, time.Millisecond*time.Duration(splitTime), cuts)

	bw := NewPacket(nil)
	bw.Write(TCPRemoteSectorSplit)
	bw.Write(car.CarID)
	bw.Write(splitIndex)
	bw.Write(splitTime)
	bw.Write(cuts)

	t.state.BroadcastOthersTCP(bw, car.CarID)

	split := Split{
		Index: splitIndex,
		Time:  splitTime,
		Cuts:  cuts,
	}

	car.CompleteSector(split)

	go func() {
		err = t.plugin.OnSectorCompleted(car.Copy(), split)

		if err != nil {
			t.logger.WithError(err).Error("On sector completed plugin returned an error")
		}
	}()

	return nil
}

func (t SectorSplitMessageHandler) MessageType() MessageType {
	return TCPRemoteSectorSplit
}
