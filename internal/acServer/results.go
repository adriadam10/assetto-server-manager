package acServer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

const CurrentResultsVersion = 2

// @TODO extend! -top speed on best lap
func (ss *ServerState) GenerateResults() *SessionResults {
	var result []*SessionResult
	var cars []*SessionCar
	var events []*SessionEvent
	var laps []*SessionLap

	for _, entrant := range ss.entryList {
		sessionDriver := SessionDriver{
			GUID:      entrant.Driver.GUID,
			GuidsList: entrant.GUIDs(),
			Name:      entrant.Driver.Name,
			Nation:    entrant.Driver.Nation,
			Team:      entrant.Driver.Team,
		}

		cars = append(cars, &SessionCar{
			BallastKG:  entrant.Ballast,
			CarID:      int(entrant.CarID),
			Driver:     sessionDriver,
			Model:      entrant.Model,
			Restrictor: entrant.Restrictor,
			Skin:       entrant.Skin,
		})
	}

	for _, leaderboardLine := range ss.Leaderboard() {
		carID := int(leaderboardLine.Car.CarID)

		sessionDriver := &SessionDriver{
			GUID:      leaderboardLine.Car.Driver.GUID,
			GuidsList: leaderboardLine.Car.GUIDs(),
			Name:      leaderboardLine.Car.Driver.Name,
			Nation:    leaderboardLine.Car.Driver.Nation,
			Team:      leaderboardLine.Car.Driver.Team,
		}

		// laps and events are within entrant.sessionData
		result = append(result, &SessionResult{
			BallastKG:  leaderboardLine.Car.Ballast,
			BestLap:    int(leaderboardLine.Car.BestLap().LapTime.Milliseconds()),
			CarID:      carID,
			CarModel:   leaderboardLine.Car.Model,
			DriverGUID: leaderboardLine.Car.Driver.GUID,
			DriverName: leaderboardLine.Car.Driver.Name,
			Restrictor: leaderboardLine.Car.Restrictor,
			TotalTime:  int(leaderboardLine.Car.TotalRaceTime().Milliseconds()),
		})

		for _, lap := range leaderboardLine.Car.SessionData.Laps {
			var sectors []int

			for _, sector := range lap.Sectors {
				sectors = append(sectors, int(sector.Milliseconds()))
			}

			laps = append(laps, &SessionLap{
				BallastKG:  lap.Ballast,
				CarID:      carID,
				CarModel:   leaderboardLine.Car.Model,
				Cuts:       lap.Cuts,
				DriverGUID: lap.DriverGUID,
				DriverName: lap.DriverName,
				LapTime:    int(lap.LapTime.Milliseconds()),
				Restrictor: lap.Restrictor,
				Sectors:    sectors,
				Timestamp:  int(lap.CompletedTime.Unix()),
				Tyre:       lap.Tyre,
			})
		}

		for _, event := range leaderboardLine.Car.SessionData.Events {
			var typeString string
			var otherCarID int

			otherDriver := &SessionDriver{}

			switch event.EventType {
			case EventTypeOtherCar:
				typeString = "COLLISION_WITH_CAR"

				otherEntrant, err := ss.GetCarByID(event.OtherCarID)

				if err != nil {
					logrus.WithError(err).Errorf("Could not find other entrant for collision")
					continue
				}

				otherCarID = int(event.OtherCarID)

				otherDriver = &SessionDriver{
					GUID:      otherEntrant.Driver.GUID,
					GuidsList: otherEntrant.GUIDs(),
					Name:      otherEntrant.Driver.Name,
					Nation:    otherEntrant.Driver.Nation,
					Team:      otherEntrant.Driver.Team,
				}
			case EventTypeEnvironment:
				typeString = "COLLISION_WITH_ENV"

				otherCarID = -1
			default:
				typeString = "UNKNOWN_EVENT"

				otherCarID = -1
			}

			events = append(events, &SessionEvent{
				CarID:         carID,
				Driver:        sessionDriver,
				ImpactSpeed:   event.Speed,
				OtherCarID:    otherCarID,
				OtherDriver:   otherDriver,
				RelPosition:   &event.RelativePosition,
				Type:          typeString,
				WorldPosition: &event.Position,
			})
		}
	}

	resultDate := time.Now()

	return &SessionResults{
		Version:     CurrentResultsVersion,
		Cars:        cars,
		Events:      events,
		Laps:        laps,
		Result:      result,
		TrackConfig: ss.raceConfig.TrackLayout,
		TrackName:   ss.raceConfig.Track,
		Type:        ss.currentSession.SessionType.ResultsString(),
		Date:        resultDate,
		SessionFile: fmt.Sprintf("%d_%d_%d_%d_%d_%s.json", resultDate.Year(), resultDate.Month(), resultDate.Day(), resultDate.Hour(), resultDate.Minute(), ss.currentSession.SessionType.ResultsString()),
	}
}

// saveResults saves the results to the disk.
func saveResults(basePath string, results *SessionResults) error {
	path := filepath.Join(basePath, "results", results.SessionFile)

	logrus.Infof("Saving session results for '%s' to: %s", results.Type, results.SessionFile)

	file, err := os.Create(path)

	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "\t")

	return encoder.Encode(results)
}

type SessionResults struct {
	Version     int              `json:"Version"`
	Cars        []*SessionCar    `json:"Cars"`
	Events      []*SessionEvent  `json:"Events"`
	Laps        []*SessionLap    `json:"Laps"`
	Result      []*SessionResult `json:"Result"`
	TrackConfig string           `json:"TrackConfig"`
	TrackName   string           `json:"TrackName"`
	Type        string           `json:"Type"`
	Date        time.Time        `json:"Date"`
	SessionFile string           `json:"SessionFile"`
}

type SessionCar struct {
	BallastKG  float32       `json:"BallastKG"`
	CarID      int           `json:"CarId"`
	Driver     SessionDriver `json:"Driver"`
	Model      string        `json:"Model"`
	Restrictor float32       `json:"Restrictor"`
	Skin       string        `json:"Skin"`
}

type SessionDriver struct {
	GUID      string   `json:"Guid"`
	GuidsList []string `json:"GuidsList"`
	Name      string   `json:"Name"`
	Nation    string   `json:"Nation"`
	Team      string   `json:"Team"`
}

type SessionEvent struct {
	CarID         int            `json:"CarId"`
	Driver        *SessionDriver `json:"Driver"`
	ImpactSpeed   float32        `json:"ImpactSpeed"`
	OtherCarID    int            `json:"OtherCarId"`
	OtherDriver   *SessionDriver `json:"OtherDriver"`
	RelPosition   *Vector3F      `json:"RelPosition"`
	Type          string         `json:"Type"`
	WorldPosition *Vector3F      `json:"WorldPosition"`
}

type SessionLap struct {
	BallastKG  float32 `json:"BallastKG"`
	CarID      int     `json:"CarId"`
	CarModel   string  `json:"CarModel"`
	Cuts       int     `json:"Cuts"`
	DriverGUID string  `json:"DriverGuid"`
	DriverName string  `json:"DriverName"`
	LapTime    int     `json:"LapTime"`
	Restrictor float32 `json:"Restrictor"`
	Sectors    []int   `json:"Sectors"`
	Timestamp  int     `json:"Timestamp"`
	Tyre       string  `json:"Tyre"`
}

type SessionResult struct {
	BallastKG  float32 `json:"BallastKG"`
	BestLap    int     `json:"BestLap"`
	CarID      int     `json:"CarId"`
	CarModel   string  `json:"CarModel"`
	DriverGUID string  `json:"DriverGuid"`
	DriverName string  `json:"DriverName"`
	Restrictor float32 `json:"Restrictor"`
	TotalTime  int     `json:"TotalTime"`
}
