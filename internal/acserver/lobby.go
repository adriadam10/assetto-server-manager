package acserver

import (
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

	mutex        sync.Mutex
	isRegistered bool
}

func NewLobby(state *ServerState, logger Logger) *Lobby {
	return &Lobby{
		state:  state,
		logger: logger,
	}
}

const (
	lobbyPortForwardingError  = "ERROR,INVALID SERVER,CHECK YOUR PORT FORWARDING SETTINGS"
	lobbyRegistrationAttempts = 10
)

func (l *Lobby) Try(description string, fn func() error) error {
	var err error

	for i := 0; i < lobbyRegistrationAttempts; i++ {
		err = fn()

		if err == nil {
			l.logger.Infof("%s succeeded (attempt %d of %d)", description, i+1, lobbyRegistrationAttempts)
			return nil
		}

		l.logger.WithError(err).Errorf("Could not: %s (attempt %d of %d)", description, i+1, lobbyRegistrationAttempts)
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

	q := r.URL.Query()
	q.Add("name", l.state.serverConfig.Name)
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

func (l *Lobby) UpdateSessionDetails(timeLeft int) error {
	l.mutex.Lock()

	if !l.isRegistered {
		l.mutex.Unlock()
		if err := l.Register(); err != nil {
			return err
		}
		l.mutex.Lock()
	}

	defer l.mutex.Unlock()

	sessionUpdateRequest, err := l.buildSessionUpdateRequest(timeLeft)

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

func (l *Lobby) buildSessionUpdateRequest(timeLeft int) (*http.Request, error) {
	r, err := http.NewRequest(http.MethodGet, lobbyPingURL, nil)

	if err != nil {
		return nil, err
	}

	q := r.URL.Query()
	q.Add("session", strconv.Itoa(int(l.state.currentSession.SessionType)))

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
