package acServer

import (
	"fmt"
	"net"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewEntryListManager(t *testing.T) {
	state := &ServerState{
		raceConfig: &EventConfig{
			PickupModeEnabled: false,
		},
		entryList:    EntryList{},
		serverConfig: &ServerConfig{},
	}

	NewEntryListManager(state, logrus.New())

	if !state.raceConfig.PickupModeEnabled {
		t.Errorf("Expected pickup mode to be forcibly enabled due to no Booking session, but it wasn't")
	}
}

type connectCarTest struct {
	name                 string
	pickup               bool
	locked               bool
	numEntrants          int
	numLockedEntrants    int
	numConnectedEntrants int

	driversToConnect []connectCarDriver
}

type connectCarDriver struct {
	Driver

	requestModel string
	succeed      bool
}

func (cct *connectCarTest) entryList() EntryList {
	e := EntryList{}

	totalLockedEntrants := 0
	totalConnectedEntrants := 0

	for i := 0; i < cct.numEntrants; i++ {
		c := &Car{
			Driver: Driver{
				Name: fmt.Sprintf("Driver %d", i),
				Team: "",
			},
			CarID: CarID(i),
			Model: connectCarTestCars[i%len(connectCarTestCars)],
		}

		if totalLockedEntrants < cct.numLockedEntrants {
			c.Driver.GUID = fmt.Sprintf("%d", i)
			totalLockedEntrants++
		}

		if totalConnectedEntrants < cct.numConnectedEntrants {
			c.Connection = NewConnection(&net.TCPConn{})
			c.Connection.udpAddr = &net.UDPAddr{}
			totalConnectedEntrants++
		}

		e = append(e, c)
	}

	return e
}

var connectCarTests = []connectCarTest{
	{
		name:                 "free car choice, no locked entrants, none connected",
		pickup:               true,
		locked:               false,
		numEntrants:          20,
		numLockedEntrants:    0,
		numConnectedEntrants: 0,
		driversToConnect: []connectCarDriver{
			{
				Driver: Driver{Name: "Driver 0", GUID: "0"}, requestModel: connectCarTestCars[0], succeed: true,
			},
		},
	},
	{
		name:                 "free car choice, no entrants, 3 connected",
		pickup:               true,
		locked:               false,
		numEntrants:          20,
		numLockedEntrants:    0,
		numConnectedEntrants: 3,
		driversToConnect: []connectCarDriver{
			{
				Driver: Driver{Name: "Driver 100", GUID: "100"}, requestModel: connectCarTestCars[2], succeed: true,
			},
		},
	},
	{
		name:                 "free car choice, no entrants, all cars have connected",
		pickup:               true,
		locked:               false,
		numEntrants:          20,
		numLockedEntrants:    0,
		numConnectedEntrants: 20,
		driversToConnect: []connectCarDriver{
			{
				Driver: Driver{Name: "Driver 30", GUID: "30"}, requestModel: connectCarTestCars[0], succeed: false,
			},
			{
				Driver: Driver{Name: "Driver 40", GUID: "40"}, requestModel: connectCarTestCars[1], succeed: false,
			},
		},
	},
	{
		name:                 "locked entrylist, 3 connected",
		pickup:               true,
		locked:               true,
		numEntrants:          9,
		numLockedEntrants:    9,
		numConnectedEntrants: 0,
		driversToConnect: []connectCarDriver{
			{
				Driver: Driver{Name: "Driver 8", GUID: "8"}, requestModel: connectCarTestCars[2], succeed: true,
			},
			{
				Driver: Driver{Name: "Driver 41", GUID: "41"}, requestModel: connectCarTestCars[1], succeed: false,
			},
		},
	},
	{
		name:                 "locked entrylist, car attempting to connect is already connected",
		pickup:               true,
		locked:               true,
		numEntrants:          5,
		numLockedEntrants:    5,
		numConnectedEntrants: 3,
		driversToConnect: []connectCarDriver{
			{
				Driver: Driver{Name: "Driver 1", GUID: "1"}, requestModel: connectCarTestCars[1], succeed: false,
			},
		},
	},
}

var connectCarTestCars = []string{"ks_ferrari_f2004", "ks_mazda_mx5_cup", "ks_porsche_911_r"}

func TestEntryListManager_ConnectCar(t *testing.T) {
	for _, test := range connectCarTests {
		t.Run(test.name, func(t *testing.T) {
			state := &ServerState{
				entryList: test.entryList(),
				raceConfig: &EventConfig{
					PickupModeEnabled: test.pickup,
					LockedEntryList:   test.locked,
					Cars:              connectCarTestCars,
				},
				serverConfig: &ServerConfig{},
			}

			for i, e := range state.entryList {
				t.Logf("%d %s %s %s\n", i, e.Driver.Name, e.Driver.GUID, e.Model)
			}

			em := NewEntryListManager(state, logrus.New())

			for _, driver := range test.driversToConnect {
				car, err := em.ConnectCar(&net.TCPConn{}, driver.Driver, driver.requestModel, false)

				if driver.succeed {
					if err != nil || car == nil {
						t.Logf("Expected driver: %s / %s to succeed in connecting, they did not", driver.Name, driver.GUID)
						t.Fail()
						continue
					}

					if car.Model != driver.requestModel {
						t.Logf("Driver: %s did not get the car they requested, got: %s, wanted %s", driver.Name, car.Model, driver.requestModel)
						t.Fail()
					}
				} else {
					if car != nil || err == nil {
						t.Logf("Expected driver: %s / %s to fail in connecting, they did not", driver.Name, driver.GUID)
						t.Fail()
					}
				}
			}
		})
	}

	t.Run("Pickup mode: Car connects, disconnects, and is given the same slot on reconnecting", func(t *testing.T) {
		entryList := EntryList{
			{
				Driver: Driver{
					Name: "Driver A",
					GUID: "A",
				},
				CarID: 0,
				Model: "ks_ferrari_f2004",
			},
			{
				Driver: Driver{
					Name: "",
					GUID: "",
				},
				CarID: 1,
				Model: "ks_ferrari_f2004",
			},
			{
				Driver: Driver{
					Name: "Driver E",
					GUID: "E",
				},
				CarID: 2,
				Model: "ks_ferrari_f2004",
			},
			{
				Driver: Driver{
					Name: "",
					GUID: "",
				},
				CarID: 3,
				Model: "ks_ferrari_f2004",
			},
		}

		state := &ServerState{
			entryList: entryList,
			raceConfig: &EventConfig{
				PickupModeEnabled: true,
				LockedEntryList:   false,
				Cars:              connectCarTestCars,
			},
			serverConfig: &ServerConfig{},
		}

		em := NewEntryListManager(state, logrus.New())
		car, err := em.ConnectCar(&net.TCPConn{}, Driver{Name: "Driver Connect", GUID: "C"}, "ks_ferrari_f2004", false)

		if err != nil {
			t.Error(err)
			return
		}

		// disconnect
		car.Connection = Connection{}

		car2, err := em.ConnectCar(&net.TCPConn{}, Driver{Name: "Driver Connect", GUID: "C"}, "ks_ferrari_f2004", false)

		if err != nil {
			t.Error(err)
			return
		}

		if car2.CarID != car.CarID {
			t.Errorf("Expected new car id to be: %d, was: %d", car.CarID, car2.CarID)
			return
		}
	})

	t.Run("Pickup mode: Car connects, disconnects, new car connects and is given the old car's slot", func(t *testing.T) {
		entryList := EntryList{
			{
				Driver: Driver{
					Name: "Driver A",
					GUID: "A",
				},
				CarID: 0,
				Model: "ks_ferrari_f2004",
			},
			{
				Driver: Driver{
					Name: "",
					GUID: "",
				},
				CarID: 1,
				Model: "ks_ferrari_f2004",
			},
			{
				Driver: Driver{
					Name: "Driver E",
					GUID: "E",
				},
				CarID: 2,
				Model: "ks_ferrari_f2004",
			},
		}

		state := &ServerState{
			entryList: entryList,
			raceConfig: &EventConfig{
				PickupModeEnabled: true,
				LockedEntryList:   false,
				Cars:              connectCarTestCars,
			},
			serverConfig: &ServerConfig{},
		}

		em := NewEntryListManager(state, logrus.New())
		car, err := em.ConnectCar(&net.TCPConn{}, Driver{Name: "Driver Connect", GUID: "C"}, "ks_ferrari_f2004", false)

		if err != nil {
			t.Error(err)
			return
		}

		// disconnect
		car.Connection = Connection{}

		car2, err := em.ConnectCar(&net.TCPConn{}, Driver{Name: "Driver Connect", GUID: "D"}, "ks_ferrari_f2004", false)

		if err != nil {
			t.Error(err)
			return
		}

		if car2.CarID != car.CarID {
			t.Errorf("Expected new car id to be: %d, was: %d", car.CarID, car2.CarID)
			return
		}
	})

}

type bookingDriver struct {
	name, guid, car, skin string
	success               bool
}

type bookCarTest struct {
	name           string
	maxClients     int
	numFilledSlots int

	drivers []bookingDriver
}

func (t *bookCarTest) entryList() EntryList {
	var e EntryList

	for i := 0; i < t.numFilledSlots; i++ {
		c := &Car{
			Driver: Driver{
				Name: fmt.Sprintf("Driver %d", i),
				Team: "",
				GUID: fmt.Sprintf("%d", i),
			},
			CarID: CarID(i),
			Model: connectCarTestCars[i%len(connectCarTestCars)],
		}
		e = append(e, c)
	}

	return e
}

var bookCarTests = []bookCarTest{
	{
		name:           "normal booking, space for 3 new drivers to book",
		maxClients:     9,
		numFilledSlots: 6,
		drivers: []bookingDriver{
			{
				name:    "New Driver",
				guid:    "30",
				car:     connectCarTestCars[0],
				skin:    "some_skin",
				success: true,
			},
			{
				name:    "New Driver 2",
				guid:    "31",
				car:     connectCarTestCars[2],
				skin:    "some_skin_3",
				success: true,
			},
			{
				name:    "New Driver",
				guid:    "888",
				car:     connectCarTestCars[1],
				skin:    "some_skin_2",
				success: true,
			},
		},
	},
	{
		name:           "no free slots",
		maxClients:     5,
		numFilledSlots: 5,
		drivers: []bookingDriver{
			{
				name:    "New Driver",
				guid:    "30",
				car:     connectCarTestCars[0],
				skin:    "some_skin",
				success: false,
			},
		},
	},
	{
		name:           "one entrant books twice, changes car",
		maxClients:     5,
		numFilledSlots: 4,
		drivers: []bookingDriver{
			{
				name:    "New Driver",
				guid:    "30",
				car:     connectCarTestCars[0],
				skin:    "some_skin",
				success: true,
			},
			{
				name:    "New Driver 2",
				guid:    "33",
				car:     connectCarTestCars[2],
				skin:    "some_skin_new",
				success: false,
			},
			{
				name:    "New Driver",
				guid:    "30",
				car:     connectCarTestCars[2],
				skin:    "some_skin_test",
				success: true,
			},
		},
	},
}

func TestEntryListManager_BookCar(t *testing.T) {
	for _, test := range bookCarTests {
		t.Run(test.name, func(t *testing.T) {
			state := &ServerState{
				entryList: test.entryList(),
				raceConfig: &EventConfig{
					PickupModeEnabled: false,
					LockedEntryList:   true,
					Cars:              connectCarTestCars,
					MaxClients:        test.maxClients,
					Sessions: Sessions{
						{
							SessionType: SessionTypeBooking,
							Time:        15,
							IsOpen:      FreeJoin,
						},
					},
				},
				serverConfig: &ServerConfig{},
			}

			em := NewEntryListManager(state, logrus.New())

			for i, e := range state.entryList {
				t.Logf("%d %s %s %s\n", i, e.Driver.Name, e.Driver.GUID, e.Model)
			}

			for _, driver := range test.drivers {
				car, err := em.BookCar(Driver{Name: driver.name, GUID: driver.guid}, driver.car, driver.skin)

				if driver.success {
					if err != nil || car == nil {
						t.Errorf("Expected to be able to book driver: %s/%s, but couldn't", driver.name, driver.guid)
						continue
					}

					if car.Model != driver.car || car.Skin != driver.skin || car.Driver.GUID != driver.guid || car.Driver.Name != driver.name {
						t.Errorf("Car assignment failed for driver: %s/%s", driver.name, driver.guid)
					}
				} else {
					if err == nil || car != nil {
						t.Errorf("Expected not to be able to book driver: %s/%s, but did", driver.name, driver.guid)
					}
				}
			}
		})
	}
}

func TestEntryListManager_UnBookCar(t *testing.T) {
	var e EntryList

	for i := 0; i < 10; i++ {
		c := &Car{
			Driver: Driver{
				Name: fmt.Sprintf("Driver %d", i),
				Team: "",
				GUID: fmt.Sprintf("%d", i),
			},
			CarID: CarID(i),
			Model: connectCarTestCars[i%len(connectCarTestCars)],
		}

		e = append(e, c)
	}

	em := NewEntryListManager(&ServerState{entryList: e,
		raceConfig: &EventConfig{
			PickupModeEnabled: false,
			LockedEntryList:   true,
			Cars:              connectCarTestCars,
			MaxClients:        15,
			Sessions: Sessions{
				{
					SessionType: SessionTypeBooking,
					Time:        15,
					IsOpen:      FreeJoin,
				},
			},
		},
		serverConfig: &ServerConfig{},
	}, logrus.New())

	t.Run("Found car, removed successfully", func(t *testing.T) {
		err := em.UnBookCar("2")

		if err != nil {
			t.Errorf("Could not unbook car, err %s", err)
			return
		}

		if len(em.state.entryList) != 9 {
			t.Errorf("Expected entrylist len to be 9, got: %d", len(em.state.entryList))
		}
	})

	t.Run("Car did not exist, not removed", func(t *testing.T) {
		err := em.UnBookCar("300")

		if err == nil {
			t.Errorf("Expected error while unbooking car, did not get one")
			return
		}

		if len(em.state.entryList) != 9 {
			t.Errorf("Expected entrylist len to be 9, got: %d", len(em.state.entryList))
		}
	})
}
