package acServer

import (
	"net"
)

type EntryListMessageHandler struct {
	state  *ServerState
	logger Logger
}

func NewEntryListMessageHandler(state *ServerState, logger Logger) *EntryListMessageHandler {
	return &EntryListMessageHandler{
		state:  state,
		logger: logger,
	}
}

// entry list is paged - max 10 entrants per page
const entryListPageSize uint8 = 10

func (e EntryListMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	from := p.ReadUint8()

	to := from + entryListPageSize

	if to > uint8(len(e.state.entryList)) {
		to = uint8(len(e.state.entryList))
	}

	e.logger.Debugf("Serving EntryList to client from: %d -> %d", from, to)

	entrants := e.state.entryList[from:to]

	o := NewPacket(nil)
	o.Write(TCPMessageEntryListPage)
	o.Write(from)
	o.Write(uint8(len(entrants)))

	entrantIndex := from

	for _, entrant := range entrants {
		o.Write(entrant.CarID)
		o.WriteString(entrant.Model)
		o.WriteString(entrant.Skin)
		o.WriteString(entrant.Driver.Name)
		o.WriteString(entrant.Driver.Team)
		o.WriteString(entrant.Driver.Nation)
		o.Write(entrant.SpectatorMode)
		o.Write(entrant.DamageZones)

		entrantIndex++
	}

	return o.WriteTCP(conn)
}

func (e EntryListMessageHandler) MessageType() MessageType {
	return TCPMessageEntryList
}
