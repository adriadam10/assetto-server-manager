package acserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/tdewolff/minify"
	"github.com/tdewolff/minify/html"
)

type HTTP struct {
	server *http.Server
	logger Logger

	port             uint16
	state            *ServerState
	sessionManager   *SessionManager
	entryListManager *EntryListManager
}

func NewHTTP(port uint16, state *ServerState, sessionManager *SessionManager, entryListManager *EntryListManager, logger Logger) *HTTP {
	return &HTTP{
		port:             port,
		state:            state,
		sessionManager:   sessionManager,
		entryListManager: entryListManager,
		logger:           logger,
	}
}

func (h *HTTP) Listen() error {
	h.logger.Infof("HTTP server listening on port: %d", h.port)

	h.server = &http.Server{
		Handler: h.Router(),
		Addr:    fmt.Sprintf(":%d", h.port),
	}

	go func() {
		err := h.server.ListenAndServe()

		if err == http.ErrServerClosed {
			return
		} else if err != nil {
			h.logger.WithError(err).Errorf("Could not start HTTP server")
		}
	}()

	return nil
}

func (h *HTTP) Router() http.Handler {
	router := chi.NewRouter()
	router.Mount("/INFO", http.HandlerFunc(h.Info))
	router.Mount("/ENTRY", http.HandlerFunc(h.TimeTable))
	router.Mount("/JSON|{guid}", http.HandlerFunc(h.EntryList))
	router.Mount("/JSON|", http.HandlerFunc(h.EntryList))
	router.Mount("/SUB|{params}", http.HandlerFunc(h.BookCar))
	router.Mount("/UNSUB|{guid}", http.HandlerFunc(h.UnBookCar))
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		h.logger.Debugf("Could not find HTTP response for URL: %s", r.URL.String())

		http.NotFound(w, r)
	})

	return router
}

type HTTPEntryList struct {
	Cars []*HTTPEntryListCar
}

type HTTPEntryListCar struct {
	Model           string
	Skin            string
	DriverName      string
	DriverTeam      string
	DriverNation    string
	IsConnected     bool
	IsRequestedGUID bool
	IsEntryList     bool
}

func (h *HTTP) EntryList(w http.ResponseWriter, r *http.Request) {
	requestedGUID := chi.URLParam(r, "guid")

	e := &HTTPEntryList{
		Cars: make([]*HTTPEntryListCar, 0),
	}

	for _, car := range h.state.entryList {
		e.Cars = append(e.Cars, &HTTPEntryListCar{
			Model:           car.Model,
			Skin:            car.Skin,
			DriverName:      car.Driver.Name,
			DriverTeam:      car.Driver.Team,
			DriverNation:    car.Driver.Nation,
			IsConnected:     car.IsConnected(),
			IsRequestedGUID: car.Driver.GUID == requestedGUID,
			IsEntryList:     !car.WasBooked,
		})
	}

	w.Header().Add("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(e)
}

func (h *HTTP) Info(w http.ResponseWriter, r *http.Request) {
	currentSession := h.sessionManager.GetCurrentSession()

	var timeLeft int

	if currentSession.Laps > 0 {
		timeLeft = h.sessionManager.RemainingLaps()
	} else {
		timeLeft = int(h.sessionManager.RemainingSessionTime().Seconds())
	}

	w.Header().Add("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(&HTTPSessionInfo{
		UDPPort:                    h.state.serverConfig.UDPPort,
		TCPPort:                    h.state.serverConfig.TCPPort,
		HTTPPort:                   h.state.serverConfig.HTTPPort,
		Name:                       h.state.serverConfig.Name,
		Clients:                    h.state.entryList.NumConnected(),
		MaxClients:                 h.state.raceConfig.MaxClients,
		Track:                      h.state.raceConfig.LobbyTrackName(),
		Cars:                       h.state.raceConfig.Cars,
		TimeOfDay:                  int(h.state.raceConfig.SunAngle),
		Session:                    h.sessionManager.GetSessionIndex(),
		SessionTypes:               h.state.raceConfig.SessionTypes(),
		Durations:                  h.state.raceConfig.SessionDurations(),
		TimeLeftOfSessionInSeconds: timeLeft,
		Country:                    [2]string{"na", "na"}, // it seems this indicates to the game to determine the GeoIP itself
		HasPassword:                h.state.serverConfig.Password != "",
		Timestamp:                  0,
		JSON:                       nil,
		RaceHasLaps:                h.state.raceConfig.RaceHasLaps(),
		RaceHasExtraLap:            h.state.raceConfig.RaceExtraLap,
		PickupMode:                 h.state.raceConfig.PickupModeEnabled,
		RaceIsTimed:                !h.state.raceConfig.RaceHasLaps(),
		Pit:                        h.state.raceConfig.HasMandatoryPit(),
		InvertedGrid:               int(h.state.raceConfig.ReversedGridRacePositions),
	})
}

type HTTPSessionInfo struct {
	IP                         string    `json:"ip"`
	UDPPort                    uint16    `json:"port"`
	TCPPort                    uint16    `json:"tport"`
	HTTPPort                   uint16    `json:"cport"`
	Name                       string    `json:"name"`
	Clients                    int       `json:"clients"`
	MaxClients                 int       `json:"maxclients"`
	Track                      string    `json:"track"`
	Cars                       []string  `json:"cars"`
	TimeOfDay                  int       `json:"timeofday"`
	Session                    uint8     `json:"session"`
	SessionTypes               []int     `json:"sessiontypes"`
	Durations                  []int     `json:"durations"`
	TimeLeftOfSessionInSeconds int       `json:"timeleft"`
	Country                    [2]string `json:"country"`
	HasPassword                bool      `json:"pass"`
	Timestamp                  int64     `json:"timestamp"`
	JSON                       *struct{} `json:"json"` // @TODO literally no idea
	RaceHasLaps                bool      `json:"l"`
	PickupMode                 bool      `json:"pickup"`
	RaceIsTimed                bool      `json:"timed"`
	RaceHasExtraLap            bool      `json:"extra"`
	Pit                        bool      `json:"pit"`
	InvertedGrid               int       `json:"inverted"`
}

const (
	bookingParamSeparator = "|"

	bookingIncorrectPassword = "INCORRECT PASSWORD"
	bookingSyntaxError       = "SYNTAX_ERROR"
	bookingServerFull        = "SERVER FULL"
	bookingIllegalCar        = "ILLEGAL CAR"
	bookingUnknownGUID       = "UNKNOWN GUID"
	bookingClosed            = "CLOSED"
	bookingOK                = "OK"
)

func (h *HTTP) BookCar(w http.ResponseWriter, r *http.Request) {
	currentSession := h.sessionManager.GetCurrentSession()

	if currentSession.SessionType != SessionTypeBooking {
		h.logger.Warnf("A booking request was made during non-booking mode.")
		_, _ = w.Write([]byte(bookingClosed))
		return
	}

	bookingParams := strings.Split(chi.URLParam(r, "params"), bookingParamSeparator)

	if len(bookingParams) != 6 {
		h.logger.Warnf("An invalid booking request was made.")
		_, _ = w.Write([]byte(bookingSyntaxError))
		return
	}

	model := bookingParams[0]
	skin := bookingParams[1]
	name := bookingParams[2]
	team := bookingParams[3]
	guid := bookingParams[4]
	password := bookingParams[5]

	if password != h.state.serverConfig.Password && password != h.state.serverConfig.AdminPassword {
		_, _ = w.Write([]byte(bookingIncorrectPassword))
		return
	}

	driver := Driver{
		Name: name,
		Team: team,
		GUID: guid,
	}

	if len(h.state.entryList) >= h.state.raceConfig.MaxClients {
		_, _ = w.Write([]byte(bookingServerFull))
		return
	}

	timeRemaining := h.sessionManager.RemainingSessionTime()

	if car, err := h.entryListManager.BookCar(driver, model, skin); err == ErrCarNotFound {
		_, _ = w.Write([]byte(fmt.Sprintf("%s,%d", bookingIllegalCar, int(timeRemaining.Seconds()))))
		return
	} else if err != nil {
		_, _ = w.Write([]byte(fmt.Sprintf("%s,%d", bookingSyntaxError, int(timeRemaining.Seconds()))))
		return
	} else {
		h.logger.Infof("Successfully booked car for driver: %s", car.String())
	}

	_, _ = w.Write([]byte(fmt.Sprintf("%s,%d", bookingOK, int(timeRemaining.Seconds()))))
}

func (h *HTTP) UnBookCar(w http.ResponseWriter, r *http.Request) {
	guid := chi.URLParam(r, "guid")
	err := h.entryListManager.UnBookCar(guid)

	if err != nil {
		_, _ = w.Write([]byte(bookingUnknownGUID))
		return
	}

	h.logger.Infof("Successfully un-booked car for driver: %s", guid)

	_, _ = w.Write([]byte(bookingOK))
}

func (h *HTTP) TimeTable(w http.ResponseWriter, r *http.Request) {
	currentSession := h.sessionManager.GetCurrentSession()

	var timeLeft time.Duration
	var lapsLeft int
	var timedEvent bool

	if currentSession.Laps > 0 {
		timedEvent = false

		lapsLeft = h.sessionManager.RemainingLaps()
	} else {
		timedEvent = true

		timeLeft = h.sessionManager.RemainingSessionTime()
	}

	buf := new(bytes.Buffer)

	err := timeTableTemplate.Execute(buf, timeTableData{
		SessionType: currentSession.SessionType,
		SessionName: currentSession.Name,
		TimeLeft:    timeLeft,
		LapsLeft:    lapsLeft,
		TimedEvent:  timedEvent,
		EntryList:   h.state.entryList,
		Leaderboard: h.state.Leaderboard(currentSession.SessionType),
	})

	if err != nil {
		h.logger.WithError(err).Errorf("Could not render timetable template")
	}

	// KMR requires HTML to be minified so it can do pattern matching.
	minifier := &html.Minifier{KeepEndTags: true}
	_ = minifier.Minify(minify.New(), w, buf, nil)
}

type timeTableData struct {
	SessionType SessionType
	SessionName string
	TimedEvent  bool
	TimeLeft    time.Duration
	LapsLeft    int
	EntryList   EntryList
	Leaderboard []*LeaderboardLine
}

func formatLapTimeDuration(d time.Duration) string {
	if d == 0 || d == maximumLapTime {
		return "--:--:---"
	}

	mins := int(d / time.Minute)
	d -= time.Duration(mins) * time.Minute
	secs := int(d / time.Second)
	d -= time.Duration(secs) * time.Second
	milli := d.Milliseconds()

	return fmt.Sprintf("%d:%02d:%03d", mins, secs, milli)
}

func formatSessionTimeDuration(d time.Duration) string {
	if d == 0 {
		return "--:--"
	}

	mins := int(d / time.Minute)
	d -= time.Duration(mins) * time.Minute
	secs := int(d / time.Second)
	negative := ""

	if secs < 0 {
		secs = -secs

		negative = "-"
	}

	if mins < 0 {
		mins = -mins

		negative = "-"
	}

	return fmt.Sprintf("%s%d:%02d", negative, mins, secs)
}

var (
	timeTableTemplateFuncMap = template.FuncMap{
		"FormatLapTime":     formatLapTimeDuration,
		"FormatSessionTime": formatSessionTimeDuration,
		"add":               func(a CarID, b int) int { return int(a) + b },
	}

	timeTableTemplate = template.Must(template.New("template").Funcs(timeTableTemplateFuncMap).Parse(timeTableHTML))
)

const timeTableHTML = `
<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
<title>Assetto Corsa Server: Entry List</title>
</head>
<body>
<p>Session: {{ $.SessionType.ResultsString }} [{{ $.SessionName }}], {{ if $.TimedEvent }}TIME LEFT: {{ FormatSessionTime $.TimeLeft }}{{ else }}LAPS REMAINING: {{ $.LapsLeft }}{{ end }}</p>
<p>Entry List</p>
<table>
	<tr>
		<td>ID</td>
		<td>Driver</td>
		<td>Car</td>
		<td>Team</td>
		<td>Ping</td>
		<td>Laps</td>
		<td>Last</td>
		<td>Best</td>
		<td>Total</td>
		<td>Guid</td>
	</tr>
	{{- range $index, $car := $.EntryList -}}
		<tr>
			<td>{{- add $car.CarID 1 -}}</td>
			<td>{{- $car.Driver.Name -}}</td>
			<td>{{- $car.Model -}}</td>
			<td>{{- $car.Driver.Team -}}</td>
			<td>
				{{- $ping := $car.Connection.Ping -}}

				{{- if eq $ping 0 -}}
					DC
				{{- else -}}
					{{- $ping -}}
				{{- end -}}
			</td>
			<td>{{- $car.SessionData.LapCount -}}</td>
			<td>
				{{- FormatLapTime $car.LastLap.LapTime -}}
			</td>
			<td>
				{{- FormatLapTime ($car.BestLap $.SessionType).LapTime -}}
			</td>
			<td>
				{{- FormatLapTime $car.TotalConnectionTime -}}
			</td>
			<td>
				{{- $car.Driver.GUID -}}
			</td>
		</tr>
	{{- end -}}
</table>
<table>
	<tr>
		<td>POS</td>
		<td>Driver</td>
		<td>Car</td>
		<td>Team</td>
		<td>Laps</td>
		<td>Time</td>
		<td>Diff</td>
	</tr>

	{{- range $pos, $line := $.Leaderboard -}}
		<tr>
			<td>{{- $pos -}}</td>
			<td>{{- $line.Car.Driver.Name -}}</td>
			<td>{{- $line.Car.Model -}}</td>
			<td>{{- $line.Car.Driver.Team -}}</td>
			<td>{{- $line.NumLaps -}}</td>
			<td>{{- FormatLapTime $line.Time -}}</td>
			<td>+{{- FormatLapTime $line.GapToLeader -}}</td>
		</tr>
	{{- end -}}
</table>
<p>Throughput: 0kBs</p>
</body>
`

func (h *HTTP) Close() error {
	h.logger.Debugf("Closing HTTP listener")

	if h.server == nil {
		return nil
	}

	return h.server.Close()
}
