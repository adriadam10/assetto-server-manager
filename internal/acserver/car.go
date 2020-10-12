package acserver

import (
	"fmt"
	"net"
	"sort"
	"sync"
	"time"
)

type CarID uint8

const ServerCarID CarID = 0xFF

type CarInfo struct {
	Driver  Driver   `json:"driver"`
	Drivers []Driver `json:"drivers"`

	CarID      CarID   `json:"car_id" yaml:"car_id"`
	Model      string  `json:"model" yaml:"model"`
	Skin       string  `json:"skin" yaml:"skin"`
	Ballast    float32 `json:"ballast" yaml:"ballast"`
	Restrictor float32 `json:"restrictor" yaml:"restrictor"`
	FixedSetup string  `json:"fixed_setup" yaml:"fixed_setup"`

	SpectatorMode uint8 `json:"-"`
	IsAdmin       bool  `json:"-"`

	Tyres       string     `json:"-"`
	DamageZones [5]float32 `json:"-"`

	SessionData SessionData `json:"-"`

	// PluginStatus is sent only to the plugin. It is used so that positional
	// updates work even when a Qualifying session is in Solo mode
	PluginStatus CarUpdate `json:"-"`
}

type Car struct {
	CarInfo

	Connection Connection `json:"-"`
	Status     CarUpdate  `json:"-"`

	mutex sync.RWMutex
}

type Driver struct {
	Name     string    `json:"name" yaml:"name"`
	Team     string    `json:"team" yaml:"team"`
	GUID     string    `json:"guid" yaml:"guid"`
	JoinTime int64     `json:"-"`
	LoadTime time.Time `json:"-"`
	Nation   string    `json:"-"`
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

	hasSentFirstUpdate   bool
	hasUpdateToBroadcast bool

	hasFailedChecksum bool
	priorities        map[CarID]int
	jumpPacketCount   map[CarID]int

	chatLimiter *time.Ticker
}

const chatLimit = time.Second * 4

func NewConnection(tcpConn net.Conn) Connection {
	chatLimiter := time.NewTicker(chatLimit)

	return Connection{
		tcpConn:         tcpConn,
		PingCache:       make([]int32, numberOfPingsForAverage),
		priorities:      make(map[CarID]int),
		jumpPacketCount: make(map[CarID]int),
		chatLimiter:     chatLimiter,
	}
}

type SessionData struct {
	Laps            []*Lap
	Sectors         []Split
	Events          []*ClientEvent
	LapCount        int
	HasExtraLapToGo bool
	P2PCount        int16
	MandatoryPit    bool

	hasCompletedSession bool
}

func (c *Car) HasGUID(guid string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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

func (c *Car) AssociateUDPAddress(addr net.Addr) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Connection.udpAddr = addr
}

func (c *Car) ChangeTyres(tyres string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Tyres = tyres
}

// UpdatePriorities uses the distance of every Car on track from this Car and ranks them between 0 and 3. The rank
// defines how many updates can be skipped when updating this Car about the position of the other Cars.
func (c *Car) UpdatePriorities(entryList EntryList) {
	distances := make(map[CarID]float64)

	var carIDs []CarID

	for _, otherCar := range entryList {
		if !otherCar.IsConnected() || !otherCar.HasSentFirstUpdate() || otherCar == c {
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
	c.mutex.Lock()
	defer c.mutex.Unlock()

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
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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

func (c *Car) SwapDrivers(newDriver Driver, conn Connection, isAdmin bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Connection = conn
	c.IsAdmin = isAdmin

	if newDriver.GUID == c.Driver.GUID {
		// the current driver was the last driver
		return
	}

	previousDriver := c.Driver
	c.Driver = newDriver

	if previousDriver.GUID == "" {
		return
	}

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
	c.mutex.Lock()
	defer c.mutex.Unlock()

	var sectors []time.Duration

	for _, sector := range lap.Splits {
		sectors = append(sectors, time.Duration(sector)*time.Millisecond)
	}

	l := &Lap{
		LapTime:       time.Duration(lap.LapTime) * time.Millisecond,
		Cuts:          int(lap.Cuts),
		Sectors:       sectors,
		CompletedTime: time.Now(),
		Tyres:         c.Tyres,
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
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	bestLap := &Lap{
		LapTime: maximumLapTime,
	}

	for _, lap := range c.SessionData.Laps {
		if lap.LapTime != 0 && lap.Cuts == 0 && lap.LapTime < bestLap.LapTime {
			bestLap = lap
		}
	}

	return bestLap
}

func (c *Car) TotalRaceTime() time.Duration {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if len(c.SessionData.Laps) == 0 {
		return &Lap{}
	}

	return c.SessionData.Laps[len(c.SessionData.Laps)-1]
}

func (c *Car) FirstLap() *Lap {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

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
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return fmt.Sprintf("CarID: %d, Name: %s, GUID: %s, Model: %s", c.CarID, c.Driver.Name, c.Driver.GUID, c.Model)
}

func (c *Car) HasSentFirstUpdate() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Connection.hasSentFirstUpdate
}

func (c *Car) SetHasSentFirstUpdate(t bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Connection.hasSentFirstUpdate = t
}

func (c *Car) Copy() CarInfo {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.CarInfo
}

const numberOfPingsForAverage = 50

func (c *Car) UpdatePing(time int64, theirTime, timeOffset uint32) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Connection.CurrentPingIndex >= numberOfPingsForAverage {
		c.Connection.CurrentPingIndex = 0
	}

	c.Connection.TargetTimeOffset = uint32(time) - timeOffset
	c.Connection.PingCache[c.Connection.CurrentPingIndex] = int32(uint32(time) - theirTime)
	c.Connection.CurrentPingIndex++

	pingSum := int32(0)
	numPings := int32(0)

	for _, ping := range c.Connection.PingCache {
		if ping > 0 {
			pingSum += ping
			numPings++
		}
	}

	if numPings <= 0 || pingSum <= 0 {
		c.Connection.Ping = 0
	} else {
		c.Connection.Ping = pingSum / numPings
	}
}

func (c *Car) AddEvent(event *ClientEvent) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.SessionData.Events = append(c.SessionData.Events, event)
}

func (c *Car) HasUpdateToBroadcast() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Connection.hasUpdateToBroadcast
}

func (c *Car) SetHasUpdateToBroadcast(b bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Connection.hasUpdateToBroadcast = b
}

func (c *Car) CompleteSector(split Split) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	splitFound := false

	for i := range c.SessionData.Sectors {
		if split.Index == c.SessionData.Sectors[i].Index {
			c.SessionData.Sectors[i] = split

			splitFound = true
			break
		}
	}

	if !splitFound {
		c.SessionData.Sectors = append(c.SessionData.Sectors, split)
	}
}

func (c *Car) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Connection.tcpConn != nil
}

func (c *Car) CloseConnection() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Connection.chatLimiter.Stop()
	c.Connection.tcpConn = nil
	c.Connection.udpAddr = nil
	c.Connection = Connection{}
}

func (c *Car) ClearSessionData() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.SessionData = SessionData{}
}

func (c *Car) SetStatus(carUpdate CarUpdate, fullAssign bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if fullAssign {
		c.Status = carUpdate
	} else {
		// solo sessions still require a timestamp and sequence number so that
		// car positions can be correctly checked
		c.Status.Timestamp = carUpdate.Timestamp
		c.Status.Sequence = carUpdate.Sequence
	}
}

func (c *Car) SetPluginStatus(carUpdate CarUpdate) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.PluginStatus = carUpdate
}

func (c *Car) SetHasFailedChecksum(b bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Connection.hasFailedChecksum = b
}

func (c *Car) HasFailedChecksum() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Connection.hasFailedChecksum
}

func (c *Car) AdjustTimeOffset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Status.Timestamp += c.Connection.TimeOffset

	diff := int(c.Connection.TargetTimeOffset) - int(c.Connection.TimeOffset)

	var v13, v14 int

	if diff >= 0 {
		v13 = diff
		v14 = diff
	} else {
		v14 = int(c.Connection.TimeOffset) - int(c.Connection.TargetTimeOffset)
	}

	if v13 > 0 || v13 == 0 && v14 > 1000 {
		c.Connection.TimeOffset = c.Connection.TargetTimeOffset
	} else if v13 == 0 && v14 < 3 || v13 < 0 {
		c.Connection.TimeOffset = c.Connection.TargetTimeOffset
	} else {
		if diff > 0 {
			c.Connection.TimeOffset = c.Connection.TimeOffset + 3
		}

		if diff < 0 {
			c.Connection.TimeOffset = c.Connection.TimeOffset - 3
		}
	}
}

func (c *Car) SetLoadedTime(t time.Time) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Driver.LoadTime = t
}

func (c *Car) HasCompletedSession() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.SessionData.hasCompletedSession
}

func (c *Car) SetHasCompletedSession(b bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.SessionData.hasCompletedSession = b
}

func (c *Car) GetLaps() []*Lap {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.SessionData.Laps
}

func (c *Car) LapCount() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.SessionData.LapCount
}
