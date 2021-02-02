package acsm

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type Debugger struct {
	store       Store
	process     ServerProcess
	healthCheck *HealthCheck
}

func NewDebugger(store Store, process ServerProcess, healthCheck *HealthCheck) *Debugger {
	return &Debugger{store: store, process: process, healthCheck: healthCheck}
}

func (d *Debugger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Disposition", fmt.Sprintf(`attachment;filename="acsm_debug_bundle_%s.zip"`, time.Now().Format("2006-01-02_15_04")))
	w.Header().Add("Content-Type", "application/zip")

	if err := d.BuildDebugInfo(w); err != nil {
		logrus.WithError(err).Error("Could not build debug information")
		http.Error(w, "Could not build debug information", http.StatusInternalServerError)
		return
	}
}

func (d *Debugger) BuildDebugInfo(w io.Writer) (err error) {
	z := zip.NewWriter(w)
	defer func() {
		closeErr := z.Close()

		if err == nil {
			err = closeErr
		}
	}()

	healthCheck := d.healthCheck.buildHealthCheckResponse()
	event := d.process.Event()

	entryList := event.GetEntryList().ToACServerConfig()
	eventConfig := event.GetRaceConfig().ToACConfig()
	serverConfig, _ := d.store.LoadServerOptions()

	customChecksums, _ := d.store.LoadCustomChecksums()
	realPenaltyOptions, _ := d.store.LoadRealPenaltyOptions()
	strackerOptions, _ := d.store.LoadStrackerOptions()
	kissMyRankOptions, _ := d.store.LoadKissMyRankOptions()
	auditLogs, _ := d.store.GetAuditEntries()

	if err := d.addJSONFileToZip(z, "process_event.json", event); err != nil {
		return err
	}

	if activeChampionship, ok := event.(*ActiveChampionship); ok {
		championship, _ := d.store.LoadChampionship(activeChampionship.ChampionshipID.String())

		if err := d.addJSONFileToZip(z, "active_championship.json", championship); err != nil {
			return err
		}
	}

	if activeRaceWeekend, ok := event.(*ActiveRaceWeekend); ok {
		raceWeekend, _ := d.store.LoadRaceWeekend(activeRaceWeekend.RaceWeekendID.String())

		if err := d.addJSONFileToZip(z, "active_race_weekend.json", raceWeekend); err != nil {
			return err
		}

		if activeRaceWeekend.IsChampionship() {
			championship, _ := d.store.LoadChampionship(activeRaceWeekend.ChampionshipID.String())

			if err := d.addJSONFileToZip(z, "active_championship.json", championship); err != nil {
				return err
			}
		}
	}

	if err := d.addJSONFileToZip(z, "health_check.json", healthCheck); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "entry_list.json", entryList); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "event_config.json", eventConfig); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "server_config.json", serverConfig); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "custom_checksums.json", customChecksums); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "real_penalty.json", realPenaltyOptions); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "stracker.json", strackerOptions); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "kmr.json", kissMyRankOptions); err != nil {
		return err
	}

	if err := d.addJSONFileToZip(z, "audit.json", auditLogs); err != nil {
		return err
	}

	c := *config
	c.Steam.Password = "_redacted_"

	if err := d.addJSONFileToZip(z, "servermanager_config.json", c); err != nil {
		return err
	}

	serverLogs := d.process.Logs()
	managerLogs := logOutput.String()
	pluginLogs := pluginsOutput.String()

	if err := d.addLogsToZip(z, "server.log", serverLogs); err != nil {
		return err
	}

	if err := d.addLogsToZip(z, "manager.log", managerLogs); err != nil {
		return err
	}

	if err := d.addLogsToZip(z, "plugin.log", pluginLogs); err != nil {
		return err
	}

	return nil
}

func (d *Debugger) addJSONFileToZip(z *zip.Writer, filename string, data interface{}) error {
	f, err := z.Create(filename)

	if err != nil {
		return err
	}

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")

	return enc.Encode(data)
}

func (d *Debugger) addLogsToZip(z *zip.Writer, filename string, data string) error {
	f, err := z.Create(filename)

	if err != nil {
		return err
	}

	_, err = f.Write([]byte(data))

	return err
}
