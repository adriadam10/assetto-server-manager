package acServer

import (
	"fmt"
	"net"
	"sort"
	"time"
)

type CarID uint8

const ServerCarID CarID = 0xFF

type Car struct {
	Driver  Driver   `json:"driver"`
	Drivers []Driver `json:"drivers"`

	CarID      CarID   `json:"car_id" yaml:"car_id"`
	Model      string  `json:"model" yaml:"model"`
	Skin       string  `json:"skin" yaml:"skin"`
	Ballast    float32 `json:"ballast" yaml:"ballast"`
	Restrictor float32 `json:"restrictor" yaml:"restrictor"`
	FixedSetup string  `json:"fixed_setup" yaml:"fixed_setup"`

	SpectatorMode uint8
	IsAdmin       bool

	Tyres       string
	DamageZones [5]float32

	HasUpdateToBroadcast bool

	Connection  Connection
	Status      CarUpdate
	SessionData SessionData
}

type Driver struct {
	Name     string `json:"name" yaml:"name"`
	Team     string `json:"team" yaml:"team"`
	GUID     string `json:"guid" yaml:"guid"`
	JoinTime int64
	LoadTime time.Time
	Nation   string
}

type Connection struct {
	tcpConn net.Conn
	udpAddr net.Addr

	Ping             int32
	TargetTimeOffset uint32
	TimeOffset       uint32
	LastPingTime     time.Time

	PingCache        []int32
	CurrentPingIndex int

	HasSentFirstUpdate bool
	FailedChecksum     bool
	priorities         map[CarID]int
	jumpPacketCount    map[CarID]int
}

func NewConnection(tcpConn net.Conn) Connection {
	return Connection{
		tcpConn:         tcpConn,
		PingCache:       make([]int32, numberOfPingsForAverage),
		priorities:      make(map[CarID]int),
		jumpPacketCount: make(map[CarID]int),
	}
}

func (c *Connection) Close() {
	closeTCPConnection(c.tcpConn)

	c.tcpConn = nil
	c.udpAddr = nil
}

type SessionData struct {
	Laps                []*Lap
	Events              []*ClientEvent
	LapCount            int
	HasCompletedSession bool
	HasExtraLapToGo     bool
	P2PCount            int16
	MandatoryPit        bool
}

func (c *Car) HasGUID(guid string) bool {
	if c.Driver.GUID == guid {
		return true
	}

	for _, driver := range c.Drivers {
		if driver.GUID == guid {
			return true
		}
	}

	return false
}

// UpdatePriorities uses the distance of every Car on track from this Car and ranks them between 0 and 3. The rank
// defines how many updates can be skipped when updating this Car about the position of the other Cars.
func (c *Car) UpdatePriorities(entryList EntryList) {
	distances := make(map[CarID]float64)

	var carIDs []CarID

	for _, otherCar := range entryList {
		if !otherCar.IsConnected() || !otherCar.Connection.HasSentFirstUpdate || otherCar == c {
			continue
		}

		distanceToCar := c.Status.Position.DistanceTo(otherCar.Status.Position)

		distances[otherCar.CarID] = distanceToCar
		carIDs = append(carIDs, otherCar.CarID)
	}

	sort.Slice(carIDs, func(i, j int) bool {
		carI := carIDs[i]
		carJ := carIDs[j]

		return distances[carI] < distances[carJ]
	})

	for index, carID := range carIDs {
		switch {
		case index < 5:
			c.Connection.priorities[carID] = 0
		case index < 10:
			c.Connection.priorities[carID] = 1
		case index < 15:
			c.Connection.priorities[carID] = 2
		default:
			c.Connection.priorities[carID] = 3
		}
	}
}

// ShouldSendUpdate returns true if the number of jumps since the last packet send is greater than otherCar's
// priority to this Car. Calling ShouldSendUpdate increases the jump packet count.
func (c *Car) ShouldSendUpdate(otherCar *Car) bool {
	priority, ok := c.Connection.priorities[otherCar.CarID]

	if !ok {
		priority = 0
	}

	jumpPacketCount, ok := c.Connection.jumpPacketCount[otherCar.CarID]

	if !ok {
		jumpPacketCount = 0
		c.Connection.jumpPacketCount[otherCar.CarID] = 0
	}

	if jumpPacketCount >= priority {
		c.Connection.jumpPacketCount[otherCar.CarID] = 0
		return true
	}

	c.Connection.jumpPacketCount[otherCar.CarID]++
	return false
}

func (c *Car) GUIDs() []string {
	guidMap := make(map[string]bool)

	guidMap[c.Driver.GUID] = true

	for _, driver := range c.Drivers {
		guidMap[driver.GUID] = true
	}

	var out []string

	for guid := range guidMap {
		out = append(out, guid)
	}

	return out
}

func (c *Car) SwapDrivers(newDriver Driver) {
	if newDriver.GUID == c.Driver.GUID {
		// the current driver was the last driver
		return
	}

	previousDriver := c.Driver
	c.Driver = newDriver

	// add previous driver to the list of known drivers of the car
	for i, driver := range c.Drivers {
		if driver.GUID == previousDriver.GUID {
			c.Drivers[i] = previousDriver
			return
		}
	}

	c.Drivers = append(c.Drivers, previousDriver)
}

func (c *Car) AddLap(lap *LapCompleted) *Lap {
	var sectors []time.Duration

	for _, sector := range lap.Splits {
		sectors = append(sectors, time.Duration(sector)*time.Millisecond)
	}

	l := &Lap{
		LapTime:       time.Duration(lap.LapTime) * time.Millisecond,
		Cuts:          int(lap.Cuts),
		Sectors:       sectors,
		CompletedTime: time.Now(),
		Tyre:          c.Tyres,
		Restrictor:    c.Restrictor,
		Ballast:       c.Ballast,
		DriverGUID:    c.Driver.GUID,
		DriverName:    c.Driver.Name,
	}

	c.SessionData.Laps = append(c.SessionData.Laps, l)
	c.SessionData.LapCount = int(lap.LapCount)

	return l

}

// maximumLapTime is the max amount of lap time possible on the server
const maximumLapTime = 999999999 * time.Millisecond

func (c *Car) BestLap() *Lap {
	if len(c.SessionData.Laps) == 0 {
		return &Lap{
			LapTime: maximumLapTime,
		}
	}

	bestLap := c.SessionData.Laps[0]

	for _, lap := range c.SessionData.Laps {
		if lap.LapTime != 0 && lap.LapTime < bestLap.LapTime {
			bestLap = lap
		}
	}

	return bestLap
}

func (c *Car) TotalRaceTime() time.Duration {
	if len(c.SessionData.Laps) == 0 {
		return time.Duration(0)
	}

	var out time.Duration

	for _, lap := range c.SessionData.Laps {
		out += lap.LapTime
	}

	return out
}

func (c *Car) LastLap() *Lap {
	if len(c.SessionData.Laps) == 0 {
		return &Lap{}
	}

	return c.SessionData.Laps[len(c.SessionData.Laps)-1]
}

func (c *Car) FirstLap() *Lap {
	if len(c.SessionData.Laps) == 0 {
		return &Lap{}
	}

	return c.SessionData.Laps[0]
}

func (c *Car) TotalConnectionTime() time.Duration {
	firstLap := c.FirstLap()

	if firstLap.LapTime == 0 {
		return 0
	}

	return time.Since(firstLap.CompletedTime.Add(-firstLap.LapTime))
}

func (c *Car) String() string {
	return fmt.Sprintf("CarID: %d, Name: %s, GUID: %s, Model: %s", c.CarID, c.Driver.Name, c.Driver.GUID, c.Model)
}

func (c *Car) IsConnected() bool {
	return c.Connection.tcpConn != nil
}
