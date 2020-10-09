package acserver

import (
	"fmt"
	"math/rand"
	"sync"
)

type DynamicTrackConfig struct {
	SessionStart    int `json:"session_start" yaml:"session_start"`
	Randomness      int `json:"randomness" yaml:"randomness"`
	SessionTransfer int `json:"session_transfer" yaml:"session_transfer"`
	LapGain         int `json:"lap_gain" yaml:"lap_gain"`
}

type DynamicTrack struct {
	DynamicTrackConfig

	startingGrip      float32
	currentGrip       float32
	numLapsBeforeGain int
	numSessions       int

	logger Logger

	mutex sync.RWMutex
}

func NewDynamicTrack(logger Logger, config DynamicTrackConfig) *DynamicTrack {
	return &DynamicTrack{
		DynamicTrackConfig: config,
		logger:             logger,
	}
}

func (d *DynamicTrack) Init() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.currentGrip = float32(d.SessionStart) / 100.0
	d.numSessions = 0
	d.numLapsBeforeGain = 0
}

func (d *DynamicTrack) OnLapCompleted() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.numLapsBeforeGain++

	if d.numLapsBeforeGain == d.LapGain && d.currentGrip < 1.0 {
		d.currentGrip += 0.01

		d.logger.Debugf("Dynamic Track: %d/%d laps completed to add 1%% grip, grip is now: %.3f", d.numLapsBeforeGain, d.LapGain, d.CurrentGrip())

		d.numLapsBeforeGain = 0
	}
}

func (d *DynamicTrack) OnNewSession(sessionType SessionType) {
	if sessionType == SessionTypeBooking {
		// SessionTypeBooking does not have cars on track, so DynamicTrack is pointless.
		return
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()

	var gripAddedInPreviousSession, gripCarriedOver float32

	if d.numSessions > 0 {
		gripAddedInPreviousSession = d.currentGrip - d.startingGrip
		gripCarriedOver = gripAddedInPreviousSession * (float32(d.SessionTransfer) / 100.0)
	}

	d.startingGrip = (d.currentGrip - gripAddedInPreviousSession) + (rand.Float32() * (float32(d.Randomness) / 100.0)) + gripCarriedOver
	d.currentGrip = d.startingGrip
	d.numLapsBeforeGain = 0

	d.logger.Infof("Dynamic Track: New Session. Starting grip: %.3f, grip carried over: %.3f", d.startingGrip, gripCarriedOver)

	d.numSessions++
}

func (d *DynamicTrack) String() string {
	return fmt.Sprintf("Session Start: %d, Randomness: %d, Session Transfer: %d, Lap Gain: %d", d.SessionStart, d.Randomness, d.SessionTransfer, d.LapGain)
}

func (d *DynamicTrack) CurrentGrip() float32 {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return d.currentGrip
}
