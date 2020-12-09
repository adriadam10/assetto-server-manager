package acserver

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cj123/ini"

	"justapengu.in/acsm/pkg/acd"
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

	IsAdmin         bool `json:"-"`
	isSpectator     bool
	spectatingCarID CarID

	Tyres       string     `json:"-"`
	DamageZones [5]float32 `json:"-"`

	SessionData SessionData `json:"-"`

	// PluginStatus is sent only to the plugin. It is used so that positional
	// updates work even when a Qualifying session is in Solo mode
	PluginStatus CarUpdate `json:"-"`

	// Deprecated: SpectatorMode is not supported by the game itself
	SpectatorMode uint8 `json:"-"`

	carLoadPosition    CarUpdate
	carLoadSessionType SessionType
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
	lastUpdateReceivedAt time.Time

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

	HasCompletedSession bool
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
	c.mutex.Lock()
	defer c.mutex.Unlock()

	distances := make(map[CarID]float64)

	var carIDs []CarID

	for _, otherCar := range entryList {
		if otherCar == c || !otherCar.IsConnected() || !otherCar.HasSentFirstUpdate() {
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

	if c.Connection.priorities == nil {
		c.Connection.priorities = make(map[CarID]int)
	}

	if c.Connection.jumpPacketCount == nil {
		c.Connection.jumpPacketCount = make(map[CarID]int)
	}

	for index, carID := range carIDs {
		if c.isSpectator {
			// spectator cars see every driver with the highest priority
			c.Connection.priorities[carID] = 0
		}

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

	if c.isSpectator {
		return true
	}

	priority, ok := c.Connection.priorities[otherCar.CarID]

	if !ok {
		priority = 0
	}

	if c.Connection.jumpPacketCount == nil {
		c.Connection.jumpPacketCount = make(map[CarID]int)
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

// GUIDsWithLaps returns the set of all GUIDs which have completed one or more laps in the Car.
func (c *Car) GUIDsWithLaps() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	guidMap := make(map[string]bool)

	guidMap[c.Driver.GUID] = true

	for _, driver := range c.Drivers {
		hasLaps := false

		for _, lap := range c.SessionData.Laps {
			if lap.DriverGUID == driver.GUID {
				hasLaps = true
				break
			}
		}

		if hasLaps {
			guidMap[driver.GUID] = true
		}
	}

	var out []string

	for guid := range guidMap {
		out = append(out, guid)
	}

	return out
}

func (c *Car) SwapDrivers(newDriver Driver, conn Connection, isAdmin, isSpectator bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Connection = conn
	c.IsAdmin = isAdmin
	c.isSpectator = isSpectator

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

func (c *Car) SetIsSpectator(b bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.isSpectator = b
}

func (c *Car) IsSpectator() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.isSpectator
}

func (c *Car) SetSpectatingCarID(carID CarID) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.spectatingCarID = carID
}

func (c *Car) GetSpectatingCarID() CarID {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.spectatingCarID
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

func (c *Car) BestLap(sessionType SessionType) *Lap {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	bestLap := &Lap{
		LapTime: maximumLapTime,
	}

	for i, lap := range c.SessionData.Laps {
		if (sessionType == SessionTypeQualifying || sessionType == SessionTypePractice) && i == 0 {
			continue
		}

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

	if c.Connection.chatLimiter != nil {
		c.Connection.chatLimiter.Stop()
	}

	c.Connection.tcpConn = nil
	c.Connection.udpAddr = nil
	c.Connection = Connection{}
}

func (c *Car) ClearSessionData() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.SessionData = SessionData{}
}

func (c *Car) SetStatus(carUpdate CarUpdate) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.Status = carUpdate
}

func (c *Car) SetPluginStatus(carUpdate CarUpdate) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.PluginStatus = carUpdate
	c.Connection.lastUpdateReceivedAt = time.Now()
}

func (c *Car) GetLastUpdateReceivedTime() time.Time {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.Connection.lastUpdateReceivedAt
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

	return c.SessionData.HasCompletedSession
}

func (c *Car) SetHasCompletedSession(b bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.SessionData.HasCompletedSession = b
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

func (c *Car) SetCarLoadPosition(l CarUpdate, sessType SessionType) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.carLoadPosition.Timestamp > 0 && sessType == SessionTypeRace {
		// we already have a car load position for this car, and we don't want
		// to use their race grid spot if we can avoid it
		return
	}

	l.Velocity = Vector3F{
		X: 0,
		Y: 0,
		Z: 0,
	}

	c.carLoadPosition = l
	c.carLoadSessionType = sessType
}

func (c *Car) GetCarLoadPosition() (CarUpdate, SessionType) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.carLoadPosition, c.carLoadSessionType
}

const IERP13c = "ier_p13c"

var (
	IERP13cTyres = []string{"S1", "S2", "S3", "S4", "S5", "S6", "S7", "S8"}

	ErrCouldNotFindTyreForCar = errors.New("acserver: could not find tyres for car")
)

func FindTyreIndex(carModel, tyreName, installPath string, legalTyres map[string]bool) (int, error) {
	tyreIndexCount := 0

	if carModel == IERP13c {
		// the IER P13c's tyre information is encrypted. Hardcoded values are used in place of the normal tyre information.
		for _, tyre := range IERP13cTyres {
			if tyre == tyreName {
				return tyreIndexCount, nil
			}

			if _, available := legalTyres[tyre]; available {
				// if the tyre we just found is in the availableTyres, then increment the tyreIndexCount
				tyreIndexCount++
			}
		}

		return -1, ErrCouldNotFindTyreForCar
	}

	tyres, err := CarDataFile(carModel, "tyres.ini", installPath)

	if err != nil {
		return -1, err
	}

	defer tyres.Close()

	f, err := ini.Load(tyres)

	if err != nil {
		return -1, err
	}

	for _, section := range f.Sections() {
		if strings.HasPrefix(section.Name(), "FRONT") {
			// this is a tyre section for the front tyres
			key, err := section.GetKey("SHORT_NAME")

			if err != nil {
				return -1, err
			}

			// we found our tyre, return the tyreIndexCount
			if key.Value() == tyreName {
				return tyreIndexCount, nil
			}

			if _, available := legalTyres[key.Value()]; available {
				// if the tyre we just found is in the availableTyres, then increment the tyreIndexCount
				tyreIndexCount++
			}
		}
	}

	return -1, ErrCouldNotFindTyreForCar
}

func CarDataFile(carModel, dataFile, installPath string) (io.ReadCloser, error) {
	carDataFile := filepath.Join(installPath, "content", "cars", carModel, "data.acd")

	f, err := os.Open(carDataFile)

	if os.IsNotExist(err) {
		// this is likely an older car with a data folder
		f, err := os.Open(filepath.Join(installPath, "content", "cars", carModel, "data", dataFile))

		if err != nil {
			return nil, err
		}

		return f, nil
	} else if err != nil {
		return nil, err
	}

	defer f.Close()

	r, err := acd.NewReader(f, carModel)

	if err != nil {
		return nil, err
	}

	for _, file := range r.Files {
		if file.Name() == dataFile {
			b, err := file.Bytes()

			if err != nil {
				return nil, err
			}

			return ioutil.NopCloser(bytes.NewReader(b)), nil
		}
	}

	return nil, os.ErrNotExist
}
