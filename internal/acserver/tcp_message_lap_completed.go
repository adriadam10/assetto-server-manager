package acserver

import (
	"net"
)

type LapCompletedMessageHandler struct {
	state          *ServerState
	sessionManager *SessionManager
}

func NewLapCompletedMessageHandler(state *ServerState, sessionManager *SessionManager) *LapCompletedMessageHandler {
	return &LapCompletedMessageHandler{
		state:          state,
		sessionManager: sessionManager,
	}
}

type LapCompleted struct {
	PhysicsTime uint32
	LapTime     uint32
	NumSplits   uint8
	Splits      []uint32
	Cuts        uint8
	LapCount    uint8
	DriverGUID  string
}

func (l LapCompletedMessageHandler) OnMessage(conn net.Conn, p *Packet) error {
	lap := &LapCompleted{}

	p.Read(&lap.PhysicsTime)
	p.Read(&lap.LapTime)
	p.Read(&lap.NumSplits)

	lap.Splits = make([]uint32, lap.NumSplits)

	for i := uint8(0); i < lap.NumSplits; i++ {
		p.Read(&lap.Splits[i])
	}

	p.Read(&lap.Cuts)
	p.Read(&lap.LapCount)

	car, err := l.state.GetCarByTCPConn(conn)

	if err != nil {
		return err
	}

	return l.sessionManager.CompleteLap(car.CarID, lap, nil)
}

func (l LapCompletedMessageHandler) MessageType() MessageType {
	return TCPMessageLapCompleted
}
