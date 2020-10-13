package acserver

import (
	"testing"
	"time"
)

type reverseLeaderboardTest struct {
	name          string
	finishingGrid []reverseLeaderboardFinishingGridPos
	numToReverse  int
	expectedOrder []string
}

type reverseLeaderboardFinishingGridPos struct {
	time     time.Duration
	name     string
	numLaps  int
	finished bool
}

func TestReverseLeaderboard(t *testing.T) {
	reverseLeaderboardTests := []reverseLeaderboardTest{
		{
			name: "All cars finish, all cars reversed",
			finishingGrid: []reverseLeaderboardFinishingGridPos{
				{time: time.Second * 360, name: "P1", numLaps: 10, finished: true},
				{time: time.Second * 365, name: "P2", numLaps: 10, finished: true},
				{time: time.Second * 367, name: "P3", numLaps: 10, finished: true},
				{time: time.Second * 369, name: "P4", numLaps: 10, finished: true},
				{time: time.Second * 380, name: "P5", numLaps: 10, finished: true},
				{time: time.Second * 400, name: "P6", numLaps: 9, finished: true},
				{time: time.Second * 402, name: "P7", numLaps: 9, finished: true},
				{time: time.Second * 200, name: "P8", numLaps: 8, finished: true},
				{time: 0, name: "P9", numLaps: 0, finished: false},
			},
			numToReverse:  -1,
			expectedOrder: []string{"P8", "P7", "P6", "P5", "P4", "P3", "P2", "P1", "P9"},
		},
		{
			name: "All cars finish, 4 cars reversed",
			finishingGrid: []reverseLeaderboardFinishingGridPos{
				{time: time.Second * 360, name: "P1", numLaps: 10, finished: true},
				{time: time.Second * 365, name: "P2", numLaps: 10, finished: true},
				{time: time.Second * 367, name: "P3", numLaps: 10, finished: true},
				{time: time.Second * 369, name: "P4", numLaps: 10, finished: true},
				{time: time.Second * 380, name: "P5", numLaps: 10, finished: true},
				{time: time.Second * 400, name: "P6", numLaps: 9, finished: true},
				{time: time.Second * 402, name: "P7", numLaps: 9, finished: true},
				{time: time.Second * 200, name: "P8", numLaps: 8, finished: true},
				{time: 0, name: "P9", numLaps: 0, finished: false},
			},
			numToReverse:  4,
			expectedOrder: []string{"P4", "P3", "P2", "P1", "P5", "P6", "P7", "P8", "P9"},
		},
		{
			name:          "No cars finish, 4 reversed",
			finishingGrid: []reverseLeaderboardFinishingGridPos{},
			numToReverse:  4,
			expectedOrder: []string{},
		},
		{
			name: "Only 5 cars finish, 8 reversed",
			finishingGrid: []reverseLeaderboardFinishingGridPos{
				{time: time.Second * 360, name: "P1", numLaps: 10, finished: true},
				{time: time.Second * 365, name: "P2", numLaps: 10, finished: true},
				{time: time.Second * 367, name: "P3", numLaps: 10, finished: true},
				{time: time.Second * 369, name: "P4", numLaps: 10, finished: true},
				{time: time.Second * 380, name: "P5", numLaps: 10, finished: true},
				{time: time.Second * 400, name: "P6", numLaps: 9, finished: false},
				{time: time.Second * 402, name: "P7", numLaps: 9, finished: false},
				{time: time.Second * 200, name: "P8", numLaps: 8, finished: false},
				{time: 0, name: "P9", numLaps: 0, finished: false},
			},
			numToReverse:  8,
			expectedOrder: []string{"P5", "P4", "P3", "P2", "P1", "P6", "P7", "P8", "P9"},
		},
		{
			name: "All cars finish, none reversed",
			finishingGrid: []reverseLeaderboardFinishingGridPos{
				{time: time.Second * 360, name: "P1", numLaps: 10, finished: true},
				{time: time.Second * 365, name: "P2", numLaps: 10, finished: true},
				{time: time.Second * 367, name: "P3", numLaps: 10, finished: true},
				{time: time.Second * 369, name: "P4", numLaps: 10, finished: true},
				{time: time.Second * 380, name: "P5", numLaps: 10, finished: true},
				{time: time.Second * 400, name: "P6", numLaps: 9, finished: true},
				{time: time.Second * 402, name: "P7", numLaps: 9, finished: true},
				{time: time.Second * 200, name: "P8", numLaps: 8, finished: true},
				{time: 0, name: "P9", numLaps: 0, finished: false},
			},
			numToReverse:  0,
			expectedOrder: []string{"P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9"},
		},
	}

	for _, test := range reverseLeaderboardTests {
		t.Run(test.name, func(t *testing.T) {
			var leaderboard []*LeaderboardLine

			for _, row := range test.finishingGrid {
				leaderboard = append(leaderboard, &LeaderboardLine{
					Car: &Car{
						CarInfo: CarInfo{
							Driver: Driver{
								Name: row.name,
							},
							SessionData: SessionData{hasCompletedSession: row.finished},
						},
					},
					Time:    row.time,
					NumLaps: row.numLaps,
				})
			}

			ReverseLeaderboard(test.numToReverse, leaderboard)

			for i, line := range leaderboard {
				if line.Car.Driver.Name != test.expectedOrder[i] {
					t.Logf("Expected %s in pos: %d, got: %s", test.expectedOrder[i], i, line.Car.Driver.Name)
					t.Fail()
				}
			}
		})
	}
}
