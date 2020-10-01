package udp

import (
	"regexp"
	"time"

	"justapengu.in/acsm/internal/acserver"
)

type Event uint8

const (
	// Receive
	EventCollisionWithCar Event = 10
	EventCollisionWithEnv Event = 11
	EventNewSession       Event = 50
	EventNewConnection    Event = 51
	EventConnectionClosed Event = 52
	EventCarUpdate        Event = 53
	EventCarInfo          Event = 54
	EventEndSession       Event = 55
	EventVersion          Event = 56
	EventChat             Event = 57
	EventClientLoaded     Event = 58
	EventSessionInfo      Event = 59
	EventError            Event = 60
	EventLapCompleted     Event = 73
	EventSplitCompleted   Event = 100
	EventTyresChanged     Event = 101
)

type Message interface {
	Event() Event
}

type ServerError struct {
	error
}

func (ServerError) Event() Event {
	return EventError
}

type CarID = acserver.CarID
type DriverGUID string

type LapCompleted struct {
	CarID     CarID  `json:"CarID"`
	LapTime   uint32 `json:"LapTime"`
	Cuts      uint8  `json:"Cuts"`
	CarsCount uint8  `json:"CarsCount"`
	Tyres     string `json:"Tyres"`

	Cars []*LapCompletedCar `json:"Cars"`
}

type SplitCompleted struct {
	CarID CarID
	Index uint8
	Time  uint32
	Cuts  uint8
}

func (SplitCompleted) Event() Event {
	return EventSplitCompleted
}

func (LapCompleted) Event() Event {
	return EventLapCompleted
}

type LapCompletedCar struct {
	CarID     CarID  `json:"CarID"`
	LapTime   uint32 `json:"LapTime"`
	Laps      uint16 `json:"Laps"`
	Completed uint8  `json:"Completed"`
}

type Vec = acserver.Vector3F

type CollisionWithCar struct {
	CarID       CarID   `json:"CarID"`
	OtherCarID  CarID   `json:"OtherCarID"`
	ImpactSpeed float32 `json:"ImpactSpeed"`
	WorldPos    Vec     `json:"WorldPos"`
	RelPos      Vec     `json:"RelPos"`

	DamageZones      [5]float32 `json:"DamageZones"`
	OtherDamageZones [5]float32 `json:"OtherDamageZones"`
}

func (CollisionWithCar) Event() Event {
	return EventCollisionWithCar
}

type CollisionWithEnvironment struct {
	CarID       CarID      `json:"CarID"`
	ImpactSpeed float32    `json:"ImpactSpeed"`
	WorldPos    Vec        `json:"WorldPos"`
	RelPos      Vec        `json:"RelPos"`
	DamageZones [5]float32 `json:"DamageZones"`
}

func (CollisionWithEnvironment) Event() Event {
	return EventCollisionWithEnv
}

type SessionCarInfo struct {
	CarID      CarID      `json:"CarID"`
	DriverName string     `json:"DriverName"`
	DriverGUID DriverGUID `json:"DriverGUID"`
	CarModel   string     `json:"CarModel"`
	CarSkin    string     `json:"CarSkin"`
	Tyres      string     `json:"Tyres"`

	DriverInitials string `json:"DriverInitials"`
	CarName        string `json:"CarName"`

	EventType Event `json:"EventType"`
}

func (s SessionCarInfo) Event() Event {
	return s.EventType
}

type Chat struct {
	CarID      CarID      `json:"CarID"`
	Message    string     `json:"Message"`
	DriverGUID DriverGUID `json:"DriverGUID"` // used for driver name colour in live timings
	DriverName string     `json:"DriverName"`
	Time       time.Time  `json:"Time"`
}

func (Chat) Event() Event {
	return EventChat
}

func NewChat(message string, carID CarID, driverName string, driverGUID DriverGUID) (Chat, error) {
	// the Assetto Corsa chat seems to not cope well with non-ascii characters. remove them.
	message = regexp.MustCompile("[[:^ascii:]]").ReplaceAllLiteralString(message, "")

	return Chat{
		CarID:      carID,
		Message:    message,
		DriverGUID: driverGUID,
		DriverName: driverName,
		Time:       time.Now(),
	}, nil
}

type CarInfo struct {
	CarID       CarID      `json:"CarID"`
	IsConnected bool       `json:"IsConnected"`
	CarModel    string     `json:"CarModel"`
	CarSkin     string     `json:"CarSkin"`
	DriverName  string     `json:"DriverName"`
	DriverTeam  string     `json:"DriverTeam"`
	DriverGUID  DriverGUID `json:"DriverGUID"`
}

func (CarInfo) Event() Event {
	return EventCarInfo
}

type CarUpdate struct {
	CarID               CarID   `json:"CarID"`
	Pos                 Vec     `json:"Pos"`
	Velocity            Vec     `json:"Velocity"`
	Gear                uint8   `json:"Gear"`
	EngineRPM           uint16  `json:"EngineRPM"`
	NormalisedSplinePos float32 `json:"NormalisedSplinePos"`
}

func (CarUpdate) Event() Event {
	return EventCarUpdate
}

type EndSession string

func (EndSession) Event() Event {
	return EventEndSession
}

type Version uint8

func (Version) Event() Event {
	return EventVersion
}

type ClientLoaded CarID

func (ClientLoaded) Event() Event {
	return EventClientLoaded
}

type SessionInfo struct {
	Version             uint8                `json:"Version"`
	SessionIndex        uint8                `json:"SessionIndex"`
	CurrentSessionIndex uint8                `json:"CurrentSessionIndex"`
	SessionCount        uint8                `json:"SessionCount"`
	ServerName          string               `json:"ServerName"`
	Track               string               `json:"Track"`
	TrackConfig         string               `json:"TrackConfig"`
	Name                string               `json:"Name"`
	Type                acserver.SessionType `json:"Type"`
	Time                uint16               `json:"Time"`
	Laps                uint16               `json:"Laps"`
	WaitTime            uint16               `json:"WaitTime"`
	AmbientTemp         uint8                `json:"AmbientTemp"`
	RoadTemp            uint8                `json:"RoadTemp"`
	WeatherGraphics     string               `json:"WeatherGraphics"`
	ElapsedMilliseconds int32                `json:"ElapsedMilliseconds"`
	IsSolo              bool                 `json:"IsSolo"`

	EventType Event `json:"EventType"`
}

func (s SessionInfo) Event() Event {
	return s.EventType
}
