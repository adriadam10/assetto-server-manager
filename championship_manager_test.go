package acsm

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/udp"
)

var TestEntryList = EntryList{
	"CAR_0": {
		Name:  "",
		GUID:  "",
		Team:  "Team 1",
		Model: "rss_formula_rss_4",
		Skin:  "",
	},
	"CAR_1": {
		Name:  "",
		GUID:  "",
		Team:  "Team 1",
		Model: "rss_formula_rss_4",
		Skin:  "",
	},
	"CAR_2": {
		Name:  "",
		GUID:  "",
		Team:  "Team 2",
		Model: "rss_formula_rss_4",
		Skin:  "",
	},
	"CAR_3": {
		Name:  "",
		GUID:  "",
		Team:  "Team 2",
		Model: "rss_formula_rss_4",
		Skin:  "",
	},
	"CAR_4": {
		Name:  "",
		GUID:  "",
		Team:  "Team 2",
		Model: "rss_formula_rss_4",
		Skin:  "",
	},
	"CAR_5": {
		Name:  "",
		GUID:  "",
		Team:  "Team 2",
		Model: "rss_formula_rss_4",
		Skin:  "",
	},
}

type dummyServerProcess struct {
	doneCh chan struct{}
}

func (dummyServerProcess) Start(event RaceEvent) error {
	return nil
}

func (dummyServerProcess) Logs() string {
	return ""
}

func (d dummyServerProcess) Stop() error {
	if d.doneCh != nil {
		d.doneCh <- struct{}{}
	}
	return nil
}

func (dummyServerProcess) Restart() error {
	return nil
}

func (dummyServerProcess) IsRunning() bool {
	return true
}

func (dummyServerProcess) Event() RaceEvent {
	return &ActiveChampionship{}
}

func (dummyServerProcess) UDPCallback(message udp.Message) {
}

func (dummyServerProcess) SetPlugin(acserver.Plugin) {

}

func (d dummyServerProcess) NotifyDone(chan struct{}) {

}

func (dummyServerProcess) GetServerConfig() ServerConfig {
	return ConfigDefault()
}

var championshipEventFixtures = []string{
	"barbagello.db",
	"red-bull-ring.db",
	// @TODO fix me
	// "barbagello-no-end-sessions.db",
}

var championshipManager *ChampionshipManager

type dummyNotificationManager struct{}

func (d *dummyNotificationManager) HasNotificationReminders() bool {
	return false
}

func (d *dummyNotificationManager) GetNotificationReminders() []int {
	var reminders []int

	return reminders
}

func (d dummyNotificationManager) SendRaceWeekendReminderMessage(raceWeekend *RaceWeekend, session *RaceWeekendSession, timer int) error {
	return nil
}

func (d dummyNotificationManager) SendMessage(title string, msg string) error {
	return nil
}

func (d dummyNotificationManager) SendMessageWithLink(title string, msg string, linkText string, link *url.URL) error {
	return nil
}

func (d dummyNotificationManager) SendRaceStartMessage(config ServerConfig, event RaceEvent) error {
	return nil
}

func (d dummyNotificationManager) GetCarList(cars string) string {
	return "nil"
}

func (d dummyNotificationManager) GetTrackInfo(track string, layout string, download bool) string {
	return "nil"
}

func (d dummyNotificationManager) SendRaceScheduledMessage(event *CustomRace, date time.Time) error {
	return nil
}

func (d dummyNotificationManager) SendRaceCancelledMessage(event *CustomRace, date time.Time) error {
	return nil
}

func (d dummyNotificationManager) SendRaceReminderMessage(event *CustomRace, timer int) error {
	return nil
}

func (d dummyNotificationManager) SendChampionshipReminderMessage(championship *Championship, event *ChampionshipEvent, timer int) error {
	return nil
}

func (d dummyNotificationManager) SaveServerOptions(oldServerOpts *GlobalServerConfig, newServerOpts *GlobalServerConfig) error {
	return nil
}

func init() {
	config = &Configuration{}
	championshipManager = NewChampionshipManager(
		NewRaceManager(
			NewJSONStore(filepath.Join(os.TempDir(), "asm-race-store"), filepath.Join(os.TempDir(), "asm-race-store-shared")),
			dummyServerProcess{},
			NewCarManager(NewTrackManager(), false, false),
			NewTrackManager(),
			&dummyNotificationManager{},
			NewRaceControl(NilBroadcaster{}, nilTrackData{}, dummyServerProcess{}, testStore, NewPenaltiesManager(testStore)),
		),
		&ACSRClient{Enabled: false},
	)
}

func checkChampionshipEventCompletion(t *testing.T, championshipID string, eventID string) {
	// now look at the championship event and see if it has a start/end time
	loadedChampionship, err := championshipManager.LoadChampionship(championshipID)

	if err != nil {
		t.Error(err)
		return
	}

	event, _, err := loadedChampionship.EventByID(eventID)

	if err != nil {
		t.Error(err)
		return
	}

	if event.StartedTime.IsZero() {
		t.Logf("Invalid championship event start time (zero)")
		t.Fail()
		return
	}

	if event.CompletedTime.IsZero() {
		t.Logf("Invalid championship event completed time (zero)")
		t.Fail()
		return
	}

	for _, sess := range event.Sessions {
		if sess.StartedTime.IsZero() {
			t.Logf("Invalid session start time (zero)")
			t.Fail()
			return
		}

		if sess.CompletedTime.IsZero() {
			t.Logf("Invalid session end time (zero)")
			t.Fail()
			return
		}
	}
}
