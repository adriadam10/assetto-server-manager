package acsm

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"justapengu.in/acsm/pkg/udp"
)

func NewRaceControlDriver(carInfo udp.SessionCarInfo) *RaceControlDriver {
	driver := &RaceControlDriver{
		RaceControlDriverData: RaceControlDriverData{
			CarInfo:  carInfo,
			Cars:     make(map[string]*RaceControlCarLapInfo),
			LastSeen: time.Now(),
		},
	}

	driver.Cars[carInfo.CarModel] = NewRaceControlCarLapInfo(carInfo.CarModel)

	return driver
}

func NewRaceControlCarLapInfo(carModel string) *RaceControlCarLapInfo {
	return &RaceControlCarLapInfo{
		CarName: prettifyName(carModel, true),
	}
}

type RaceControlDriverData struct {
	CarInfo      udp.SessionCarInfo `json:"CarInfo"`
	TotalNumLaps int                `json:"TotalNumLaps"`

	ConnectedTime time.Time `json:"ConnectedTime" ts:"date"`
	LoadedTime    time.Time `json:"LoadedTime" ts:"date"`

	Position            int       `json:"Position"`
	Split               string    `json:"Split"`
	LastSeen            time.Time `json:"LastSeen" ts:"date"`
	LastPos             udp.Vec   `json:"LastPos"`
	IsInPits            bool      `json:"IsInPits"`
	NormalisedSplinePos float32   `json:"NormalisedSplinePos"`
	SteerAngle          uint8     `json:"SteerAngle"`
	StatusBytes         uint32    `json:"StatusBytes"`
	BlueFlag            bool      `json:"BlueFlag"`

	Collisions []Collision `json:"Collisions"`

	// Cars is a map of CarModel to the information for that car.
	Cars map[string]*RaceControlCarLapInfo `json:"Cars"`
}

type RaceControlDriver struct {
	RaceControlDriverData

	driverSwapContext context.Context
	driverSwapCfn     context.CancelFunc

	mutex sync.RWMutex
}

func (rcd *RaceControlDriver) CurrentCar() *RaceControlCarLapInfo {
	if car, ok := rcd.Cars[rcd.CarInfo.CarModel]; ok {
		return car
	}

	logrus.Warnf("Could not find current car for driver: %s (current car: %s)", rcd.CarInfo.DriverGUID, rcd.CarInfo.CarModel)
	return &RaceControlCarLapInfo{}
}

func (rcd *RaceControlDriver) ClearSessionInfo() {
	rcd.mutex.Lock()
	defer rcd.mutex.Unlock()

	carInfo := rcd.CarInfo
	loadedTime := rcd.LoadedTime
	connectedTime := rcd.ConnectedTime

	rcd.RaceControlDriverData = RaceControlDriverData{
		CarInfo:       carInfo,
		LoadedTime:    loadedTime,
		ConnectedTime: connectedTime,

		Cars: map[string]*RaceControlCarLapInfo{
			carInfo.CarModel: NewRaceControlCarLapInfo(carInfo.CarModel),
		},
	}
}

func (rcd *RaceControlDriver) MarshalJSON() ([]byte, error) {
	rcd.mutex.RLock()
	defer rcd.mutex.RUnlock()

	return json.Marshal(rcd.RaceControlDriverData)
}

type RaceControlCarLapInfo struct {
	TopSpeedThisLap      float64       `json:"TopSpeedThisLap"`
	TopSpeedBestLap      float64       `json:"TopSpeedBestLap"`
	TyresBestLap         string        `json:"TyreBestLap"`
	BestLap              time.Duration `json:"BestLap"`
	NumLaps              int           `json:"NumLaps"`
	LastLap              time.Duration `json:"LastLap"`
	LastLapCompletedTime time.Time     `json:"LastLapCompletedTime" ts:"date"`
	TotalLapTime         time.Duration `json:"TotalLapTime"`
	CarName              string        `json:"CarName"`

	CurrentLapSplits map[uint8]RaceControlCarSplit `json:"CurrentLapSplits"`
	BestSplits       map[uint8]RaceControlCarSplit `json:"BestLapSplits"`
}

type RaceControlCarSplit struct {
	SplitIndex    uint8         `json:"SplitIndex"`
	SplitTime     time.Duration `json:"SplitTime"`
	Cuts          uint8         `json:"Cuts"`
	IsDriversBest bool          `json:"IsDriversBest"`
	IsBest        bool          `json:"IsBest"`
}

type DriverMap struct {
	Drivers                map[udp.DriverGUID]*RaceControlDriver `json:"Drivers"`
	GUIDsInPositionalOrder []udp.DriverGUID                      `json:"GUIDsInPositionalOrder"`

	driverSortLessFunc driverSortLessFunc
	driverGroup        RaceControlDriverGroup

	rwMutex sync.RWMutex
}

type RaceControlDriverGroup int

const (
	ConnectedDrivers    RaceControlDriverGroup = 0
	DisconnectedDrivers RaceControlDriverGroup = 1
)

type driverSortLessFunc func(group RaceControlDriverGroup, driverA, driverB *RaceControlDriver) bool

func NewDriverMap(driverGroup RaceControlDriverGroup, driverSortLessFunc driverSortLessFunc) *DriverMap {
	return &DriverMap{
		Drivers:            make(map[udp.DriverGUID]*RaceControlDriver),
		driverSortLessFunc: driverSortLessFunc,
		driverGroup:        driverGroup,
	}
}

func (d *DriverMap) Each(fn func(driverGUID udp.DriverGUID, driver *RaceControlDriver) error) error {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	for _, guid := range d.GUIDsInPositionalOrder {
		driver, ok := d.Drivers[guid]

		if !ok {
			continue
		}

		err := fn(guid, driver)

		if err != nil {
			return err
		}
	}

	return nil
}

func (d *DriverMap) Get(driverGUID udp.DriverGUID) (*RaceControlDriver, bool) {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	driver, ok := d.Drivers[driverGUID]

	return driver, ok
}

func (d *DriverMap) Add(driverGUID udp.DriverGUID, driver *RaceControlDriver) {
	d.rwMutex.Lock()
	defer func() {
		d.rwMutex.Unlock()
		d.sort()
	}()

	d.Drivers[driverGUID] = driver

	for _, guid := range d.GUIDsInPositionalOrder {
		if guid == driverGUID {
			return
		}
	}

	d.GUIDsInPositionalOrder = append(d.GUIDsInPositionalOrder, driverGUID)
}

func (d *DriverMap) sort() {
	d.rwMutex.Lock()
	defer d.rwMutex.Unlock()

	sort.Slice(d.GUIDsInPositionalOrder, func(i, j int) bool {
		driverA, ok := d.Drivers[d.GUIDsInPositionalOrder[i]]

		if !ok {
			return false
		}

		driverB, ok := d.Drivers[d.GUIDsInPositionalOrder[j]]

		if !ok {
			return false
		}

		return d.driverSortLessFunc(d.driverGroup, driverA, driverB)
	})

	// correct positions
	for pos, guid := range d.GUIDsInPositionalOrder {
		driver, ok := d.Drivers[guid]

		if !ok {
			continue
		}

		driver.mutex.Lock()
		driver.Position = pos + 1
		driver.mutex.Unlock()
	}
}

func (d *DriverMap) Del(driverGUID udp.DriverGUID) {
	d.rwMutex.Lock()
	defer func() {
		d.rwMutex.Unlock()
		d.sort()
	}()

	delete(d.Drivers, driverGUID)

	for index, guid := range d.GUIDsInPositionalOrder {
		if guid == driverGUID {
			d.GUIDsInPositionalOrder = append(d.GUIDsInPositionalOrder[:index], d.GUIDsInPositionalOrder[index+1:]...)
			break
		}
	}
}

func (d *DriverMap) Len() int {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	return len(d.Drivers)
}

func (d *DriverMap) MarshalJSON() ([]byte, error) {
	d.rwMutex.RLock()
	defer d.rwMutex.RUnlock()

	return json.Marshal(struct {
		Drivers                map[udp.DriverGUID]*RaceControlDriver `json:"Drivers"`
		GUIDsInPositionalOrder []udp.DriverGUID                      `json:"GUIDsInPositionalOrder"`
	}{
		Drivers:                d.Drivers,
		GUIDsInPositionalOrder: d.GUIDsInPositionalOrder,
	})
}
