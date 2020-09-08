package acServer

import (
	"math"
	"testing"

	"github.com/sirupsen/logrus"
)

type dynamicTrackTest struct {
	dynamicTrack DynamicTrack
	sessions     []dynamicTrackSession
}

type dynamicTrackSession struct {
	sessionType             SessionType
	expectedGripAtBeginning float32
	expectedGripAtEnd       float32
	numLaps                 int
}

func TestDynamicTrack(t *testing.T) {
	dts := []dynamicTrackTest{
		{
			dynamicTrack: DynamicTrack{
				SessionStart:    90,
				Randomness:      0,
				SessionTransfer: 50,
				LapGain:         1,
			},
			sessions: []dynamicTrackSession{
				{sessionType: SessionTypePractice, numLaps: 6, expectedGripAtBeginning: 0.90, expectedGripAtEnd: 0.96},
				{sessionType: SessionTypeQualifying, numLaps: 0, expectedGripAtBeginning: 0.93, expectedGripAtEnd: 0.93},
			},
		},
		{
			dynamicTrack: DynamicTrack{
				SessionStart:    80,
				Randomness:      0,
				SessionTransfer: 0,
				LapGain:         20,
			},
			sessions: []dynamicTrackSession{
				{sessionType: SessionTypePractice, numLaps: 19, expectedGripAtBeginning: 0.80, expectedGripAtEnd: 0.80},
				{sessionType: SessionTypeQualifying, numLaps: 40, expectedGripAtBeginning: 0.80, expectedGripAtEnd: 0.82},
				{sessionType: SessionTypeRace, numLaps: 3, expectedGripAtBeginning: 0.80, expectedGripAtEnd: 0.80},
			},
		},
		{
			dynamicTrack: DynamicTrack{
				SessionStart:    80,
				Randomness:      0,
				SessionTransfer: 100,
				LapGain:         5,
			},
			sessions: []dynamicTrackSession{
				{sessionType: SessionTypePractice, numLaps: 20, expectedGripAtBeginning: 0.80, expectedGripAtEnd: 0.84},
				{sessionType: SessionTypePractice, numLaps: 40, expectedGripAtBeginning: 0.84, expectedGripAtEnd: 0.92},
				{sessionType: SessionTypePractice, numLaps: 10, expectedGripAtBeginning: 0.92, expectedGripAtEnd: 0.94},
			},
		},
		{
			dynamicTrack: DynamicTrack{
				SessionStart:    80,
				Randomness:      0,
				SessionTransfer: 25,
				LapGain:         5,
			},
			sessions: []dynamicTrackSession{
				{sessionType: SessionTypePractice, numLaps: 20, expectedGripAtBeginning: 0.80, expectedGripAtEnd: 0.84},
				{sessionType: SessionTypeQualifying, numLaps: 40, expectedGripAtBeginning: 0.81, expectedGripAtEnd: 0.89},
				{sessionType: SessionTypeQualifying, numLaps: 10, expectedGripAtBeginning: 0.83, expectedGripAtEnd: 0.85},
			},
		},
	}

	logger := logrus.New()

	for _, test := range dts {
		t.Run(test.dynamicTrack.String(), func(t *testing.T) {
			test.dynamicTrack.Init(logger)

			for i, session := range test.sessions {
				test.dynamicTrack.OnNewSession(session.sessionType)

				if !compareFloatsTolerance(test.dynamicTrack.CurrentGrip, session.expectedGripAtBeginning) {
					t.Logf("Expected session grip at beginning to be: %f, was: %f (session %d)", session.expectedGripAtBeginning, test.dynamicTrack.CurrentGrip, i)
					t.Fail()
				}

				for i := 0; i < session.numLaps; i++ {
					test.dynamicTrack.OnLapCompleted()
				}

				if !compareFloatsTolerance(test.dynamicTrack.CurrentGrip, session.expectedGripAtEnd) {
					t.Logf("Expected session grip at end to be: %f, was: %f (session %d)", session.expectedGripAtEnd, test.dynamicTrack.CurrentGrip, i)
					t.Fail()
				}
			}
		})
	}

	t.Run("Randomness", func(t *testing.T) {
		dt := DynamicTrack{
			SessionStart:    80,
			Randomness:      5,
			SessionTransfer: 25,
			LapGain:         5,
		}

		dt.Init(logger)
		dt.OnNewSession(SessionTypeQualifying)

		if dt.CurrentGrip > 0.85 || dt.CurrentGrip < 0.80 {
			t.Fail()
		}

		for i := 0; i < 10; i++ {
			dt.OnLapCompleted()
		}

		dt.OnNewSession(SessionTypeRace)

		if dt.CurrentGrip < 0.805 || dt.CurrentGrip > 0.905 {
			t.Fail()
		}
	})

	t.Run("Booking", func(t *testing.T) {
		dt := DynamicTrack{
			SessionStart:    80,
			Randomness:      0,
			SessionTransfer: 25,
			LapGain:         1,
		}

		dt.Init(logger)
		dt.OnNewSession(SessionTypeBooking)

		if dt.CurrentGrip != 0.80 {
			t.Fail()
		}
	})
}

func compareFloatsTolerance(a, b float32) bool {
	tolerance := 0.0001
	diff := math.Abs(float64(a - b))

	return diff < tolerance
}
