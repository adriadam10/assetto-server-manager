package acServer

import (
	"net"
	"testing"
)

type shouldSendUpdateTest struct {
	name string
	cars map[CarID]shouldSendUpdateCar
}

type shouldSendUpdateCar struct {
	position Vector3F

	everyN map[CarID]int
}

var shouldSendUpdateTests = []shouldSendUpdateTest{
	{
		name: "default",
		cars: map[CarID]shouldSendUpdateCar{
			10: {
				position: Vector3F{0, 0, 0},
				everyN: map[CarID]int{
					8:  1,
					4:  1,
					7:  1,
					20: 1,
					11: 1,
					0:  2,
					2:  2,
					3:  2,
					6:  2,
					9:  2,
					1:  3,
					14: 3,
					5:  3,
					12: 3,
					13: 3,
					15: 4,
					17: 4,
					19: 4,
				},
			},
			8: {
				position: Vector3F{-8, 0, 0},
				everyN: map[CarID]int{
					10: 1,
					4:  1,
					7:  1,
					20: 1,
					11: 1,
					0:  2,
					2:  2,
					3:  2,
					6:  2,
					9:  2,
					1:  3,
					14: 3,
					5:  3,
					12: 3,
					13: 3,
					15: 4,
					17: 4,
					19: 4,
				},
			},
			4: {
				position: Vector3F{-12, 0, 0},
				everyN: map[CarID]int{
					10: 2,
					8:  1,
					7:  1,
					20: 1,
					11: 1,
					0:  1,
					2:  2,
					3:  2,
					6:  2,
					9:  2,
					1:  3,
					14: 3,
					5:  3,
					12: 3,
					13: 3,
					15: 4,
					17: 4,
					19: 4,
				},
			},
			7: {
				position: Vector3F{-13, 0, 0},
				everyN: map[CarID]int{
					10: 2,
					8:  1,
					4:  1,
					20: 1,
					11: 1,
					0:  1,
					2:  2,
					3:  2,
					6:  2,
					9:  2,
					1:  3,
					14: 3,
					5:  3,
					12: 3,
					13: 3,
					15: 4,
					17: 4,
					19: 4,
				},
			},
			20: {
				position: Vector3F{-14, 0, 0},
			},
			11: {
				position: Vector3F{-16, 0, 0},
			},
			0: {
				position: Vector3F{-18, 0, 0},
			},
			2: {
				position: Vector3F{-20, 0, 0},
			},
			3: {
				position: Vector3F{-22, 0, 0},
			},
			6: {
				position: Vector3F{-24, 0, 0},
			},
			9: {
				position: Vector3F{-26, 0, 0},
			},
			1: {
				position: Vector3F{-28, 0, 0},
			},
			14: {
				position: Vector3F{-30, 0, 0},
			},
			5: {
				position: Vector3F{-32, 0, 0},
			},
			12: {
				position: Vector3F{-34, 0, 0},
			},
			13: {
				position: Vector3F{-36, 0, 0},
			},
			15: {
				position: Vector3F{-38, 0, 0},
			},
			17: {
				position: Vector3F{-40, 0, 0},
			},
			19: {
				position: Vector3F{-42, 0, 0},
				everyN: map[CarID]int{
					10: 4,
					8:  4,
					4:  4,
					7:  3,
					20: 3,
					11: 3,
					0:  3,
					2:  3,
					3:  2,
					6:  2,
					9:  2,
					1:  2,
					14: 2,
					5:  1,
					12: 1,
					13: 1,
					15: 1,
					17: 1,
				},
			},
		},
	},
}

func TestCar_ShouldSendUpdate(t *testing.T) {
	for _, test := range shouldSendUpdateTests {
		t.Run(test.name, func(t *testing.T) {
			entryList := EntryList{}

			for carID, carDetails := range test.cars {
				conn := NewConnection(&net.TCPConn{})
				conn.udpAddr = &net.UDPAddr{}
				conn.HasSentFirstUpdate = true

				car := &Car{
					CarID:      carID,
					Connection: conn,
					Status:     CarUpdate{Position: carDetails.position},
				}

				entryList = append(entryList, car)
			}

			for tick := 1; tick <= 100; tick++ {
				for _, car := range entryList {
					car.Status.Position.X += 1
					car.Status.Position.Y += 1
					car.Status.Position.Z += 1
				}

				for _, car := range entryList {
					car.UpdatePriorities(entryList)
				}

				for _, car := range entryList {
					for _, otherCar := range entryList {
						if car == otherCar {
							continue
						}

						// check the zeros
						checkEvery, ok := test.cars[car.CarID].everyN[otherCar.CarID]

						if !ok {
							continue
						}

						shouldSend := car.ShouldSendUpdate(otherCar)

						if tick%checkEvery == 0 {
							if !shouldSend {
								t.Errorf("Tick %d: Expected to send from car: %d to car: %d every: %d ticks, but didn't", tick, car.CarID, otherCar.CarID, checkEvery)
								return
							}
						} else {
							if shouldSend {
								t.Errorf("Tick %d: Expected NOT to send from car: %d to car: %d every: %d ticks, but did", tick, car.CarID, otherCar.CarID, checkEvery)
								return
							}
						}
					}
				}
			}
		})
	}

}
