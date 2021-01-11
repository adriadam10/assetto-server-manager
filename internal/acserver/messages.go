package acserver

import (
	"net"
	"time"
)

type MessageType uint8

const (
	UDPMessageConnectionInitialised MessageType = 0xC8
	UDPMessageAssociate             MessageType = 0x4E
	UDPMessageCarUpdate             MessageType = 0x46
	UDPMessageMegaPacket            MessageType = 0x48
	UDPMessagePing                  MessageType = 0xF8
	UDPMessagePong                  MessageType = 0xF9
)

const (
	// Receive
	TCPHandshakeBegin     MessageType = 0x3D
	TCPMessageEntryList   MessageType = 0x3F
	TCPMessageCarID       MessageType = 0x0D
	UDPMessageSessionInfo MessageType = 0x4F
	TCPMessageDisconnect  MessageType = 0x43

	// Send
	TCPHandshakeSuccess             MessageType = 0x3E
	TCPMessageEntryListPage         MessageType = 0x40
	TCPMessageChecksum              MessageType = 0x44
	TCPMessageTyreChange            MessageType = 0x50
	TCPHandshakeSessionClosed       MessageType = 0x6E
	TCPHandshakeUnsupportedProtocol MessageType = 0x42
	TCPHandshakeNoSlotsAvailable    MessageType = 0x45
	TCPHandshakeStillBooking        MessageType = 0x41
	TCPHandshakeBlockListed         MessageType = 0x3B
	TCPHandshakeWrongPassword       MessageType = 0x3C
	TCPHandshakeAuthFailed          MessageType = 0x6F
	TCPMandatoryPitCompleted        MessageType = 0x0E
	TCPSendWeather                  MessageType = 0x78
	TCPBroadcastClientDisconnected  MessageType = 0x4D
	TCPMessageSendChat              MessageType = 0x47
	TCPMessageLapCompleted          MessageType = 0x49
	TCPMessageCurrentSessionInfo    MessageType = 0x4A
	TCPMessageSessionCompleted      MessageType = 0x4B
	TCPMessageSessionStart          MessageType = 0x57
	TCPSendTextFile                 MessageType = 0x51
	TCPSpacer                       MessageType = 0x00
	TCPSendSetup                    MessageType = 0x52
	TCPSendDRSZone                  MessageType = 0x53
	TCPSendSunAngle                 MessageType = 0x54
	TCPMessageDamageZones           MessageType = 0x56
	TCPSendBoP                      MessageType = 0x70
	TCPMessageClientEvent           MessageType = 0x82
	TCPRemoteSectorSplit            MessageType = 0x58
	TCPCarConnected                 MessageType = 0x5A
	TCPConnectedEntrants            MessageType = 0x5B
	TCPMessageKick                  MessageType = 0x68
	TCPMessageVoteNextSession       MessageType = 0x64
	TCPMessageVoteRestartSession    MessageType = 0x65
	TCPMessageVoteKick              MessageType = 0x66
	TCPMessageVoteStep              MessageType = 0x67
	TCPMessageBroadcastChat         MessageType = 0x47
)

type TCPMessageHandler interface {
	OnMessage(conn net.Conn, p *Packet) error
	MessageType() MessageType
}

type UDPMessageHandler interface {
	OnMessage(conn net.PacketConn, addr net.Addr, p *Packet) error
	MessageType() MessageType
}

type SessionType uint8

const (
	SessionTypeBooking    SessionType = 0
	SessionTypePractice   SessionType = 1
	SessionTypeQualifying SessionType = 2
	SessionTypeRace       SessionType = 3
)

func (s SessionType) String() string {
	switch s {
	case SessionTypeBooking:
		return "Booking"
	case SessionTypePractice:
		return "Practice"
	case SessionTypeQualifying:
		return "Qualifying"
	case SessionTypeRace:
		return "Race"
	default:
		return "Unknown SessionType"
	}
}

func (s SessionType) ResultsString() string {
	switch s {
	case SessionTypeBooking:
		return "BOOK"
	case SessionTypePractice:
		return "PRACTICE"
	case SessionTypeQualifying:
		return "QUALIFY"
	case SessionTypeRace:
		return "RACE"
	default:
		return "Unknown SessionType"
	}
}

type SessionInfo struct {
	Version             uint8         `json:"Version"`
	SessionIndex        uint8         `json:"SessionIndex"`
	CurrentSessionIndex uint8         `json:"CurrentSessionIndex"`
	SessionCount        uint8         `json:"SessionCount"`
	ServerName          string        `json:"ServerName"`
	Track               string        `json:"Track"`
	TrackConfig         string        `json:"TrackConfig"`
	Name                string        `json:"Name"`
	NumMinutes          uint16        `json:"Time"`
	NumLaps             uint16        `json:"Laps"`
	WaitTime            int           `json:"WaitTime"`
	AmbientTemp         uint8         `json:"AmbientTemp"`
	RoadTemp            uint8         `json:"RoadTemp"`
	WeatherGraphics     string        `json:"WeatherGraphics"`
	ElapsedTime         time.Duration `json:"ElapsedTime"`
	SessionType         SessionType   `json:"EventType"`
	IsSolo              bool          `json:"IsSolo"`
	CurrentGrip         float32
}

type KickReason uint8

const (
	KickReasonGeneric              KickReason = 0
	KickReasonVotedToBeBanned      KickReason = 1
	KickReasonVotedToBeBlockListed KickReason = 2
	KickReasonChecksumFailed       KickReason = 3
)

type TCPKickMessage struct {
	CarID      CarID
	KickReason KickReason
}

type BlockListMode uint8

const (
	BlockListModeNormalKick BlockListMode = 0
	BlockListModeNoRejoin   BlockListMode = 1
	BlockListModeAddToList  BlockListMode = 2
)
