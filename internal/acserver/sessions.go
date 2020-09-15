package acserver

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

var serverStartTime = time.Now()

func currentTimeMillisecond() int64 {
	return time.Since(serverStartTime).Milliseconds()
}

type SessionConfig struct {
	sessionParams

	SessionType SessionType `json:"session_type" yaml:"session_type"`
	Name        string      `json:"name" yaml:"name"`
	Time        uint16      `json:"time" yaml:"time"`
	Laps        uint16      `json:"laps" yaml:"laps"`
	IsOpen      OpenRule    `json:"is_open" yaml:"is_open"`
	Solo        bool        `json:"solo" yaml:"solo"`
	WaitTime    int         `json:"wait_time" yaml:"wait_time"`

	Weather []*WeatherConfig `json:"weather"`
}

type sessionParams struct {
	startTime, moveToNextSessionAt int64
	sessionOverBroadcast           bool
	reverseGridRaceStarted         bool
	numCompletedLaps               int
}

func (s SessionConfig) FinishTime() int64 {
	if s.Laps > 0 {
		logrus.Errorf("SessionConfig.FinishTime was called on a session which has laps.")
		return 0
	}

	return s.startTime + int64(s.Time)*60*1000
}

func (s SessionConfig) String() string {
	var sessionLength string

	if s.Laps > 0 {
		sessionLength = fmt.Sprintf("%d Laps", s.Laps)
	} else {
		sessionLength = fmt.Sprintf("%d minutes", s.Time)
	}

	return fmt.Sprintf("%s - Name: %s, Length: %s, Wait Time: %ds, Open Rule: %s", s.SessionType, s.Name, sessionLength, s.WaitTime, s.IsOpen)
}

func (s SessionConfig) IsZero() bool {
	return s.SessionType == 0 && s.Name == "" && s.Time == 0 && s.Laps == 0
}

type SessionManager struct {
	state          *ServerState
	lobby          *Lobby
	plugin         Plugin
	logger         Logger
	weatherManager *WeatherManager
	serverStopFn   func() error

	baseDirectory string
}

func NewSessionManager(state *ServerState, weatherManager *WeatherManager, lobby *Lobby, plugin Plugin, logger Logger, serverStopFn func() error, baseDirectory string) *SessionManager {
	return &SessionManager{
		state:          state,
		lobby:          lobby,
		weatherManager: weatherManager,
		serverStopFn:   serverStopFn,
		plugin:         plugin,
		logger:         logger,
		baseDirectory:  baseDirectory,
	}
}

func (sm *SessionManager) NextSession(force bool) {
	var previousSessionLeaderboard []*LeaderboardLine

	if !sm.state.currentSession.IsZero() {
		if sm.state.currentSession.numCompletedLaps > 0 {
			sm.logger.Infof("Leaderboard at the end of the session '%s' is:", sm.state.currentSession.Name)

			previousSessionLeaderboard = sm.state.Leaderboard()

			for pos, leaderboardLine := range previousSessionLeaderboard {
				sm.logger.Printf("%d. %s - %s", pos, leaderboardLine.Car.Driver.Name, leaderboardLine)
			}

			results := sm.state.GenerateResults()
			err := saveResults(sm.baseDirectory, results)

			if err != nil {
				sm.logger.WithError(err).Error("Could not save results file!")
			} else {
				go func() {
					err := sm.plugin.OnEndSession(results.SessionFile)

					if err != nil {
						sm.logger.WithError(err).Error("On end session plugin returned an error")
					}
				}()
			}

			if sm.state.raceConfig.ReversedGridRacePositions != 0 && !sm.state.currentSession.reverseGridRaceStarted && int(sm.state.currentSessionIndex) == len(sm.state.raceConfig.Sessions)-1 {
				// if there are reverse grid positions, then we need to reorganise the grid
				sm.logger.Infof("Next session is reverse grid race. Resetting session params, reverse grid is:")

				sm.state.currentSession.sessionParams = sessionParams{
					reverseGridRaceStarted: true,
				}

				ReverseLeaderboard(int(sm.state.raceConfig.ReversedGridRacePositions), previousSessionLeaderboard)

				for pos, leaderboardLine := range previousSessionLeaderboard {
					sm.logger.Printf("%d. %s - %s", pos, leaderboardLine.Car.Driver.Name, leaderboardLine)
				}
			} else {
				sm.state.currentSessionIndex++
			}
		} else {
			sm.logger.Infof("Session '%s' had no completed laps. Will not save results JSON", sm.state.currentSession.Name)

			if force {
				sm.state.currentSessionIndex++
			} else {
				switch sm.state.currentSession.SessionType {
				case SessionTypeRace:
					sm.state.currentSessionIndex = 0
				case SessionTypeBooking:
					if len(sm.state.entryList) > 0 {
						sm.state.currentSessionIndex++
					}
				case SessionTypePractice:
					sm.state.currentSessionIndex++
				default:
					// current session index is left unchanged for qualifying.
				}
			}
		}

		sm.ClearSessionData()
	}

	if int(sm.state.currentSessionIndex) >= len(sm.state.raceConfig.Sessions) {
		if sm.state.raceConfig.LoopMode {
			sm.logger.Infof("Loop Mode is enabled. Server will restart from first session.")
			sm.state.raceConfig.DynamicTrack.Init(sm.logger)
			sm.state.currentSessionIndex = 0
		} else {
			_ = sm.serverStopFn()
			return
		}
	}

	sm.state.currentSession = *sm.state.raceConfig.Sessions[sm.state.currentSessionIndex]
	sm.state.currentSession.startTime = currentTimeMillisecond() + int64(sm.state.currentSession.WaitTime*1000)
	sm.state.currentSession.moveToNextSessionAt = 0
	sm.state.BroadcastSessionStart(sm.state.currentSession.startTime)

	sm.state.currentSession.sessionOverBroadcast = false

	for _, entrant := range sm.state.entryList {
		if entrant.IsConnected() {
			if err := sm.state.SendSessionInfo(entrant, previousSessionLeaderboard); err != nil {
				sm.logger.WithError(err).Error("Couldn't send session info")
			}
		}
	}

	sm.weatherManager.OnNewSession()

	sm.logger.Infof("Advanced to next session: %s", sm.state.currentSession)

	sm.state.raceConfig.DynamicTrack.OnNewSession(sm.state.currentSession.SessionType)
	sm.UpdateLobby()

	go func() {
		err := sm.plugin.OnNewSession(SessionInfo{
			Version:         CurrentResultsVersion,
			SessionIndex:    sm.state.currentSessionIndex,
			SessionCount:    uint8(len(sm.state.raceConfig.Sessions)),
			ServerName:      sm.state.serverConfig.Name,
			Track:           sm.state.raceConfig.Track,
			TrackConfig:     sm.state.raceConfig.TrackLayout,
			Name:            sm.state.currentSession.Name,
			NumMinutes:      sm.state.currentSession.Time,
			NumLaps:         sm.state.currentSession.Laps,
			WaitTime:        sm.state.currentSession.WaitTime,
			AmbientTemp:     sm.weatherManager.currentWeather.Ambient,
			RoadTemp:        sm.weatherManager.currentWeather.Road,
			WeatherGraphics: sm.weatherManager.currentWeather.GraphicsName,
			ElapsedTime:     sm.ElapsedSessionTime(),
			SessionType:     sm.state.currentSession.SessionType,
		})

		if err != nil {
			sm.logger.WithError(err).Error("On new session plugin returned an error")
		}
	}()
}

func (sm *SessionManager) loop(ctx context.Context) {
	tick := time.NewTicker(time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			sm.logger.Debugf("Stopping SessionManager Loop")
			return
		case <-tick.C:
			if sm.state.currentSession.moveToNextSessionAt == 0 && sm.CurrentSessionHasFinished() && !sm.state.currentSession.sessionOverBroadcast {
				carsAreConnecting := false

				for _, car := range sm.state.entryList {
					if car.IsConnected() && !car.Connection.HasSentFirstUpdate {
						carsAreConnecting = true
						break
					}
				}

				if carsAreConnecting {
					// don't advance sessions while cars are connecting.
					sm.logger.Debugf("Stalling session until all connecting cars are connected")
					continue
				}

				sm.state.BroadcastSessionCompleted()
				switch sm.state.currentSession.SessionType {
				case SessionTypeBooking:
					sm.state.currentSession.moveToNextSessionAt = currentTimeMillisecond()
				default:
					sm.state.currentSession.moveToNextSessionAt = currentTimeMillisecond() + int64(sm.state.raceConfig.ResultScreenTime*1000)
				}
				sm.state.currentSession.sessionOverBroadcast = true
			}

			if sm.state.currentSession.sessionOverBroadcast && currentTimeMillisecond() > sm.state.currentSession.moveToNextSessionAt {
				carsAreConnecting := false

				for _, car := range sm.state.entryList {
					if car.IsConnected() && !car.Connection.HasSentFirstUpdate {
						carsAreConnecting = true
						break
					}
				}

				if carsAreConnecting {
					// don't advance sessions while cars are connecting.
					sm.logger.Debugf("Stalling session until all connecting cars are connected")
					continue
				}

				// move to the next session when the race over packet has been sent and the results screen time has elapsed.
				sm.NextSession(false)
			}
		}
	}
}

func (sm *SessionManager) RestartSession() {
	sm.state.currentSessionIndex--
	sm.NextSession(true)
}

func (sm *SessionManager) CurrentSessionHasFinished() bool {
	switch sm.state.currentSession.SessionType {
	case SessionTypeRace:
		var remainingLaps int
		var remainingTime time.Duration

		if sm.state.currentSession.Laps > 0 {
			remainingLaps = sm.RemainingLaps()

			if remainingLaps > 0 {
				return false
			}
		} else {
			remainingTime = sm.RemainingSessionTime()

			if remainingTime >= 0 {
				return false
			}
		}

		if !sm.LeaderHasFinishedSession() {
			return false
		}

		if sm.AllCarsHaveFinishedSession() {
			sm.logger.Infof("All cars in session: %s have completed final laps. Ending session now.", sm.state.currentSession.Name)
			return true
		}

		leaderboard := sm.state.Leaderboard()

		if len(leaderboard) == 0 {
			return true
		}

		leader := leaderboard[0].Car
		raceOverTime := time.Duration(int64(sm.state.raceConfig.RaceOverTime)*1000) * time.Millisecond

		return time.Since(leader.SessionData.Laps[leader.SessionData.LapCount-1].CompletedTime) > raceOverTime
	case SessionTypeBooking:
		return sm.RemainingSessionTime() <= 0
	default:
		remainingTime := sm.RemainingSessionTime()

		if remainingTime >= 0 {
			return false
		}

		if sm.AllCarsHaveFinishedSession() {
			sm.logger.Infof("All cars in session: %s have completed final laps. Ending session now.", sm.state.currentSession.Name)
			return true
		}

		bestLapTime := sm.BestLapTimeInSession()

		if bestLapTime == maximumLapTime {
			// no laps were completed in this session
			sm.logger.Infof("Session: %s has no laps. Advancing to next session now.", sm.state.currentSession.Name)
			return true
		}

		waitTime := time.Duration(float64(bestLapTime.Milliseconds())*float64(sm.state.raceConfig.QualifyMaxWaitPercentage)/100) * time.Millisecond

		return remainingTime < -waitTime
	}
}

func (sm *SessionManager) RemainingSessionTime() time.Duration {
	return time.Duration(sm.state.currentSession.FinishTime()-currentTimeMillisecond()) * time.Millisecond
}

func (sm *SessionManager) RemainingLaps() int {
	leaderboard := sm.state.Leaderboard()
	numLapsInSession := int(sm.state.currentSession.Laps)

	if len(leaderboard) == 0 {
		return numLapsInSession
	}

	remainingLaps := numLapsInSession - leaderboard[0].NumLaps

	return remainingLaps
}

func (sm *SessionManager) LeaderHasFinishedSession() bool {
	if sm.state.entryList.NumConnected() == 0 {
		return true
	}

	leaderHasCrossedLine := false

	for pos, leaderboardLine := range sm.state.Leaderboard() {
		if pos == 0 && leaderboardLine.Car.SessionData.HasCompletedSession {
			leaderHasCrossedLine = true
			break
		}
	}

	return leaderHasCrossedLine
}

func (sm *SessionManager) AllCarsHaveFinishedSession() bool {
	if sm.state.entryList.NumConnected() == 0 {
		return true
	}

	finished := true

	for _, entrant := range sm.state.entryList {
		finished = finished && (!entrant.IsConnected() || entrant.SessionData.HasCompletedSession)
	}

	return finished
}

func (sm *SessionManager) BestLapTimeInSession() time.Duration {
	var bestLapTime time.Duration

	for _, entrant := range sm.state.entryList {
		best := entrant.BestLap()

		if bestLapTime == 0 {
			bestLapTime = best.LapTime
		}

		if best.LapTime < bestLapTime {
			bestLapTime = best.LapTime
		}
	}

	return bestLapTime
}

func (sm *SessionManager) ElapsedSessionTime() time.Duration {
	return time.Duration(currentTimeMillisecond()-sm.state.currentSession.startTime) * time.Millisecond
}

func (sm *SessionManager) ClearSessionData() {
	sm.logger.Infof("Clearing session data for all cars")
	sm.state.currentSession.sessionParams = sessionParams{}

	for _, car := range sm.state.entryList {
		car.SessionData = SessionData{}
	}
}

func (sm *SessionManager) JoinIsAllowed(guid string) bool {
	if entrant := sm.state.GetCarByGUID(guid, false); entrant != nil {
		// entrants which were previously in this race are allowed back in
		if !entrant.Driver.LoadTime.IsZero() {
			return true
		}
	}

	switch sm.state.currentSession.IsOpen {
	case NoJoin:
		return false
	case FreeJoin:
		return true
	case FreeJoinUntil20SecondsBeforeGreenLight:
		return sm.ElapsedSessionTime() <= -20*time.Second
	default:
		return true
	}
}

func (sm *SessionManager) UpdateLobby() {
	if !sm.state.serverConfig.RegisterToLobby {
		return
	}

	updateFunc := func() error {
		remaining := 0

		if sm.state.currentSession.Laps > 0 {
			remaining = sm.RemainingLaps()
		} else {
			remaining = int(sm.RemainingSessionTime().Seconds())
		}

		return sm.lobby.UpdateSessionDetails(remaining)
	}

	if err := sm.lobby.Try("Update lobby with new session", updateFunc); err != nil {
		sm.logger.WithError(err).Error("All attempts to update lobby with new session failed")
	}
}

func ReverseLeaderboard(numToReverse int, leaderboard []*LeaderboardLine) {
	if numToReverse == 0 {
		return
	}

	if numToReverse > len(leaderboard) || numToReverse < 0 {
		numToReverse = len(leaderboard)
	}

	for i, line := range leaderboard {
		if i > numToReverse {
			break
		}

		if !line.Car.SessionData.HasCompletedSession {
			numToReverse = i
			break
		}
	}

	toReverse := leaderboard[:numToReverse]

	for i := len(toReverse)/2 - 1; i >= 0; i-- {
		opp := len(toReverse) - 1 - i
		toReverse[i], toReverse[opp] = toReverse[opp], toReverse[i]
	}

	for i := 0; i < len(toReverse); i++ {
		leaderboard[i] = toReverse[i]
	}
}
