package acserver

import (
	"errors"
	"net"
	"sync"
)

type EntryList []*Car

func (el EntryList) NumConnected() int {
	i := 0

	for _, e := range el {
		if e.IsConnected() {
			i++
		}
	}

	return i
}

func (el EntryList) HasFixedSetup() bool {
	for _, entrant := range el {
		if entrant.FixedSetup != "" {
			return true
		}
	}

	return false
}

type EntryListManager struct {
	state  *ServerState
	mutex  sync.Mutex
	logger Logger
}

func NewEntryListManager(state *ServerState, logger Logger) *EntryListManager {
	if !state.raceConfig.HasSession(SessionTypeBooking) && !state.raceConfig.PickupModeEnabled {
		logger.Warnf("Server does not have a Booking session, yet PickupMode is disabled. Force enabling PickupMode.")
		state.raceConfig.PickupModeEnabled = true
	}

	return &EntryListManager{
		state:  state,
		logger: logger,
	}
}

var ErrNoAvailableSlots = errors.New("acserver: no available slots")

func (em *EntryListManager) ConnectCar(conn net.Conn, driver Driver, requestedModel string, isAdmin, isSpectator bool) (*Car, error) {
	em.mutex.Lock()
	defer em.mutex.Unlock()

	if em.state.raceConfig.LockedEntryList || em.state.raceConfig.HasSession(SessionTypeBooking) {
		// in locked entry list, the drivers guid must match the car
		for _, car := range em.state.entryList {
			if car.IsConnected() {
				continue
			}

			if car.HasGUID(driver.GUID) {
				car.SwapDrivers(driver, NewConnection(conn), isAdmin, isSpectator)

				return car, nil
			}
		}
	} else if em.state.raceConfig.PickupModeEnabled {
		// in pickup mode, any slot which is not taken can be given to a driver, so long as the car they request matches

		// look first to see if they've been in a car previously
		for _, car := range em.state.entryList {
			if car.IsConnected() {
				continue
			}

			if car.HasGUID(driver.GUID) && car.Model == requestedModel {
				car.SwapDrivers(driver, NewConnection(conn), isAdmin, isSpectator)

				return car, nil
			}
		}

		// look for 'empty' cars.
		for _, car := range em.state.entryList {
			if car.IsConnected() {
				continue
			}

			if car.Model == requestedModel {
				car.SwapDrivers(driver, NewConnection(conn), isAdmin, isSpectator)
				car.ClearSessionData() // reset laps if we've taken someone else's car.

				return car, nil
			}
		}
	}

	return nil, ErrNoAvailableSlots
}

func (em *EntryListManager) BookCar(driver Driver, model, skin string) (*Car, error) {
	car := &Car{
		CarInfo: CarInfo{
			Driver: driver,
			CarID:  CarID(len(em.state.entryList)),
			Model:  model,
			Skin:   skin,
		},
	}

	for index, existingCar := range em.state.entryList {
		if existingCar.HasGUID(driver.GUID) {
			car.CarID = existingCar.CarID
			em.state.entryList[index] = car
			return car, nil
		}
	}

	if len(em.state.entryList) >= em.state.raceConfig.MaxClients {
		return nil, ErrNoAvailableSlots
	}

	em.state.entryList = append(em.state.entryList, car)

	return car, nil
}

func (em *EntryListManager) UnBookCar(guid string) error {
	toRemove := -1

	for index, car := range em.state.entryList {
		if car.HasGUID(guid) {
			toRemove = index
			break
		}
	}

	if toRemove >= 0 {
		em.state.entryList = append(em.state.entryList[:toRemove], em.state.entryList[toRemove+1:]...)
	} else {
		return ErrCarNotFound
	}

	return nil
}

func (em *EntryListManager) AddDriverToEmptyCar(driver Driver, model string) error {
	for _, car := range em.state.entryList {
		if car.IsConnected() {
			continue
		}

		if car.Driver.GUID == "" && car.Model == model {
			car.SwapDrivers(driver, Connection{}, false, false)
			return nil
		}
	}

	return ErrNoAvailableSlots
}
