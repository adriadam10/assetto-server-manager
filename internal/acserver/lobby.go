package acserver

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Lobby struct {
	state  *ServerState
	logger Logger
	ctx    context.Context

	mutex        sync.Mutex
	isRegistered bool
}

func NewLobby(ctx context.Context, state *ServerState, logger Logger) *Lobby {
	return &Lobby{
		state:  state,
		logger: logger,
		ctx:    ctx,
	}
}

const (
	lobbyPortForwardingError  = "ERROR,INVALID SERVER,CHECK YOUR PORT FORWARDING SETTINGS"
	lobbyRegistrationAttempts = 10
)

func (l *Lobby) Try(description string, fn func() error, reportSuccess bool) error {
	var err error

	for i := 0; i < lobbyRegistrationAttempts; i++ {
		select {
		case <-l.ctx.Done():
			return nil
		default:
			err = fn()

			if err == nil {
				if reportSuccess {
					l.logger.Infof("%s succeeded (attempt %d of %d)", description, i+1, lobbyRegistrationAttempts)
				}
				return nil
			}

			l.logger.WithError(err).Errorf("Could not: %s (attempt %d of %d)", description, i+1, lobbyRegistrationAttempts)
		}
	}

	return err
}

var ErrLobbyPortForwardingFailed = errors.New("kunos: could not register to lobby - check your port forwarding settings")

func (l *Lobby) Register() error {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if l.isRegistered {
		return nil
	}

	registrationRequest, err := l.buildRegistrationRequest()

	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(registrationRequest)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	statusBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	status := string(statusBytes)

	switch status {
	case lobbyPortForwardingError:
		return ErrLobbyPortForwardingFailed
	default:
		if strings.HasPrefix(status, "OK,") {
			l.isRegistered = true
			return nil
		}

		return fmt.Errorf("kunos: lobby registration error: %s", status)
	}
}

const (
	lobbyRegistrationURL = "http://93.57.10.21/lobby.ashx/register"
	lobbyPingURL         = "http://93.57.10.21/lobby.ashx/ping"
)

func (l *Lobby) buildRegistrationRequest() (*http.Request, error) {
	r, err := http.NewRequest(http.MethodGet, lobbyRegistrationURL, nil)

	if err != nil {
		return nil, err
	}

	name := l.state.serverConfig.Name

	if len(name) > 127 {
		name = name[:127]

		l.logger.Warnf("Server name is too long for lobby! It has been automatically shortened from %d characters to %d characters", len(l.state.serverConfig.Name), len(name))
	}

	q := r.URL.Query()
	q.Add("name", name)
	q.Add("port", strconv.Itoa(int(l.state.serverConfig.UDPPort)))
	q.Add("tcp_port", strconv.Itoa(int(l.state.serverConfig.TCPPort)))
	q.Add("max_clients", strconv.Itoa(l.state.raceConfig.MaxClients))
	q.Add("track", l.state.raceConfig.LobbyTrackName())
	q.Add("cars", strings.Join(l.state.raceConfig.Cars, ","))
	q.Add("timeofday", strconv.Itoa(int(l.state.raceConfig.SunAngle)))

	var sessionTypes []string
	var sessionDurations []string
	raceSessionIsTimed := true

	for _, session := range l.state.raceConfig.Sessions {
		sessionTypes = append(sessionTypes, strconv.Itoa(int(session.SessionType)))

		if session.Laps > 0 {
			sessionDurations = append(sessionDurations, strconv.Itoa(int(session.Laps)))
			raceSessionIsTimed = false
		} else {
			sessionDurations = append(sessionDurations, strconv.Itoa(int(session.Time)))
		}
	}

	q.Add("sessions", strings.Join(sessionTypes, ","))
	q.Add("durations", strings.Join(sessionDurations, ","))
	q.Add("password", kunosBool(l.state.serverConfig.Password != ""))
	q.Add("version", strconv.Itoa(int(CurrentProtocolVersion)))
	q.Add("pickup", kunosBool(l.state.raceConfig.PickupModeEnabled))
	q.Add("autoclutch", kunosBool(l.state.raceConfig.AutoClutchAllowed))
	q.Add("abs", strconv.Itoa(int(l.state.raceConfig.ABSAllowed)))
	q.Add("tc", strconv.Itoa(int(l.state.raceConfig.TractionControlAllowed)))
	q.Add("stability", kunosBool(l.state.raceConfig.StabilityControlAllowed))
	q.Add("legal_tyres", strings.Join(l.state.raceConfig.LegalTyres, ";"))
	q.Add("fixed_setup", kunosBool(l.state.entryList.HasFixedSetup()))
	q.Add("timed", kunosBool(raceSessionIsTimed))
	q.Add("extra", kunosBool(l.state.raceConfig.RaceExtraLap))
	q.Add("pit", kunosBool(l.state.raceConfig.HasMandatoryPit()))
	q.Add("inverted", strconv.Itoa(int(l.state.raceConfig.ReversedGridRacePositions)))

	r.URL.RawQuery = q.Encode()

	return r, nil
}

func (l *Lobby) UpdateSessionDetails(currentSession SessionType, timeLeft int) error {
	l.mutex.Lock()

	if !l.isRegistered {
		l.mutex.Unlock()
		if err := l.Register(); err != nil {
			return err
		}
		l.mutex.Lock()
	}

	defer l.mutex.Unlock()

	sessionUpdateRequest, err := l.buildSessionUpdateRequest(currentSession, timeLeft)

	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(sessionUpdateRequest)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	statusBytes, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	status := string(statusBytes)

	switch status {
	case lobbyPortForwardingError:
		return ErrLobbyPortForwardingFailed
	default:
		if status == "OK" {
			return nil
		}

		return fmt.Errorf("kunos: lobby registration error: %s", status)
	}
}

func (l *Lobby) buildSessionUpdateRequest(currentSession SessionType, timeLeft int) (*http.Request, error) {
	r, err := http.NewRequest(http.MethodGet, lobbyPingURL, nil)

	if err != nil {
		return nil, err
	}

	q := r.URL.Query()

	if timeLeft < 0 {
		// kunos lobby cannot handle negative timeLeft
		timeLeft = 0
	}

	q.Add("session", strconv.Itoa(int(currentSession)))

	q.Add("timeleft", strconv.Itoa(timeLeft))

	q.Add("port", strconv.Itoa(int(l.state.serverConfig.UDPPort)))
	q.Add("clients", strconv.Itoa(l.state.entryList.NumConnected()))
	q.Add("track", l.state.raceConfig.LobbyTrackName())
	q.Add("pickup", kunosBool(l.state.raceConfig.PickupModeEnabled))

	r.URL.RawQuery = q.Encode()

	return r, nil
}

func kunosBool(b bool) string {
	if b {
		return "1"
	}

	return "0"
}
