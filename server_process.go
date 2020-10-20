package acsm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/internal/acserver/plugins"
	"justapengu.in/acsm/pkg/ai"
	"justapengu.in/acsm/pkg/pitlanedetection"

	"github.com/sirupsen/logrus"
)

const MaxLogSizeBytes = 1e6

type ServerProcess interface {
	Start(event RaceEvent, config *GlobalServerConfig) error
	Stop() error
	Restart() error
	IsRunning() bool
	Event() RaceEvent
	CurrentServerConfig() *GlobalServerConfig
	SetPlugin(acserver.Plugin)
	NotifyDone(chan struct{})
	Logs() string
	SharedPitLane() *pitlanedetection.PitLane
}

type eventStartPacket struct {
	raceEvent RaceEvent
	config    *GlobalServerConfig
}

// AssettoServerProcess manages the Assetto Corsa Server process.
type AssettoServerProcess struct {
	store                 Store
	contentManagerWrapper *ContentManagerWrapper

	start                 chan eventStartPacket
	startMutex            sync.Mutex
	started, stopped, run chan error
	notifyDoneChs         []chan struct{}
	server                *acserver.Server
	plugin                acserver.Plugin

	ctx context.Context
	cfn context.CancelFunc

	logBuffer *logBuffer

	raceEvent      RaceEvent
	serverConfig   *GlobalServerConfig
	mutex          sync.Mutex
	extraProcesses []*exec.Cmd

	sharedPitLane *pitlanedetection.PitLane
	logFile       io.WriteCloser
}

func NewAssettoServerProcess(store Store, contentManagerWrapper *ContentManagerWrapper) *AssettoServerProcess {
	sp := &AssettoServerProcess{
		start:                 make(chan eventStartPacket),
		started:               make(chan error),
		stopped:               make(chan error),
		run:                   make(chan error),
		logBuffer:             newLogBuffer(MaxLogSizeBytes),
		store:                 store,
		contentManagerWrapper: contentManagerWrapper,
		sharedPitLane: &pitlanedetection.PitLane{
			PitLaneSpline: &ai.Spline{},
			TrackSpline:   &ai.Spline{},
		},
	}

	go sp.loop()

	return sp
}

func (sp *AssettoServerProcess) Start(event RaceEvent, config *GlobalServerConfig) error {
	sp.startMutex.Lock()
	defer sp.startMutex.Unlock()

	if sp.IsRunning() {
		if err := sp.Stop(); err != nil {
			return err
		}
	}

	sp.start <- eventStartPacket{raceEvent: event, config: config}

	return <-sp.started
}

var ErrPluginConfigurationRequiresUDPPortSetup = errors.New("servermanager: kissmyrank and stracker configuration requires UDP plugin configuration in Server Options")

func (sp *AssettoServerProcess) IsRunning() bool {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	return sp.raceEvent != nil && sp.server != nil
}

var ErrServerProcessTimeout = errors.New("servermanager: server process did not stop even after manual kill. please check your server configuration")

func (sp *AssettoServerProcess) Stop() error {
	if !sp.IsRunning() || sp.server == nil {
		return nil
	}

	timeout := time.After(time.Second * 10)
	errCh := make(chan error)

	go func() {
		select {
		case err := <-sp.stopped:
			errCh <- err
			return
		case <-timeout:
			errCh <- ErrServerProcessTimeout
			return
		}
	}()

	if err := sp.server.Stop(config.Server.PersistMidSessionResults); err != nil {
		logrus.WithError(err).Error("Could not forcibly kill server")
	}

	sp.cfn()

	return <-errCh
}

func (sp *AssettoServerProcess) Restart() error {
	sp.mutex.Lock()
	raceEvent := sp.raceEvent
	config := sp.serverConfig
	sp.mutex.Unlock()

	return sp.Start(raceEvent, config)
}

func (sp *AssettoServerProcess) loop() {
	for {
		select {
		case err := <-sp.run:
			if err != nil {
				logrus.WithError(err).Warn("acServer process ended with error. If everything seems fine, you can safely ignore this error.")
			}

			select {
			case sp.stopped <- sp.onStop():
			default:
			}
		case startPacket := <-sp.start:
			sp.started <- sp.startRaceEvent(startPacket.raceEvent, startPacket.config)
		}
	}
}

func (sp *AssettoServerProcess) SetPlugin(plugin acserver.Plugin) {
	sp.plugin = plugin
}

func (sp *AssettoServerProcess) startRaceEvent(raceEvent RaceEvent, serverOptions *GlobalServerConfig) error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	logrus.Infof("Starting Server Process with event: %s", describeRaceEvent(raceEvent))

	var logOutput io.Writer

	if serverOptions.LogACServerOutputToFile {
		logDirectory := filepath.Join(ServerInstallPath, "logs", "session")

		if err := os.MkdirAll(logDirectory, 0755); err != nil {
			return err
		}

		if err := sp.deleteOldLogFiles(serverOptions.NumberOfACServerLogsToKeep); err != nil {
			return err
		}

		timestamp := time.Now().Format("2006-01-02_15-04-05")

		var err error

		sp.logFile, err = os.Create(filepath.Join(logDirectory, "output_"+timestamp+".log"))

		if err != nil {
			return err
		}

		logOutput = io.MultiWriter(sp.logBuffer, &noErrClosedWriter{w: sp.logFile})
	} else {
		logOutput = sp.logBuffer
	}

	wd, err := os.Getwd()

	if err != nil {
		return err
	}

	sp.raceEvent = raceEvent
	sp.serverConfig = serverOptions

	sp.ctx, sp.cfn = context.WithCancel(context.Background())

	logger := logrus.New()
	logger.SetOutput(logOutput)
	logger.SetLevel(logrus.GetLevel())
	logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	raceConfig := raceEvent.GetRaceConfig()

	trackMetaData, err := LoadTrackMetaDataFromName(raceConfig.Track)

	if err != nil {
		return err
	}

	layoutMetaDataForCalc := &LayoutMetaData{
		SplineCalculationDistance:    3,
		SplineCalculationMaxSpeed:    30,
		SplineCalculationMaxDistance: 4,
	}

	if _, ok := trackMetaData.Layouts[raceConfig.TrackLayout]; ok {
		layoutMetaDataForCalc = trackMetaData.Layouts[raceConfig.TrackLayout]
	}

	sp.sharedPitLane, err = pitlanedetection.NewSharedPitLane(
		ServerInstallPath,
		raceConfig.Track,
		raceConfig.TrackLayout,
		layoutMetaDataForCalc.SplineCalculationDistance,
		layoutMetaDataForCalc.SplineCalculationMaxDistance,
		layoutMetaDataForCalc.SplineCalculationMaxSpeed,
	)

	if err != nil {
		logrus.WithError(err).Errorf("Couldn't read track (%s) splines, pit lane detection disabled", raceConfig.Track)
		sp.sharedPitLane = &pitlanedetection.PitLane{
			PitLaneCapable: false,
		}
	}

	udpPluginPortsSetup := serverOptions.UDPPluginLocalPort >= 0 && serverOptions.UDPPluginAddress != "" || strings.Contains(serverOptions.UDPPluginAddress, ":")

	var activePlugins []acserver.Plugin

	activePlugins = append(
		activePlugins,
		sp.plugin,
		plugins.NewPenaltiesPlugin(sp.sharedPitLane),
	)

	if raceConfig.DriftModeEnabled {
		activePlugins = append(activePlugins, plugins.NewDriftPlugin())
	}

	if udpPluginPortsSetup {
		udpPlugin, err := plugins.NewUDPPlugin(serverOptions.UDPPluginLocalPort, serverOptions.UDPPluginAddress)

		if err != nil {
			return err
		}

		activePlugins = append(activePlugins, udpPlugin)
	}

	var checksums []acserver.CustomChecksumFile

	if len(raceConfig.ForcedApps) > 0 {
		forcedApps, err := sp.store.LoadCustomChecksums()

		if err != nil {
			logrus.WithError(err).Errorf("Could not load forced plugins")
		}

		if forcedApps != nil {
			for _, appID := range raceConfig.ForcedApps {
				for _, app := range forcedApps.Entries {
					if app.ID.String() == appID {
						// apply this custom forced app
						checksums = append(checksums, acserver.CustomChecksumFile{
							Name:     app.Name,
							Filename: app.Filepath,
							MD5:      app.Checksum,
						})
					}
				}
			}
		}
	}

	sp.server, err = acserver.NewServer(
		sp.ctx,
		ServerInstallPath,
		serverOptions.ToACServerConfig(),
		raceEvent.GetRaceConfig().ToACConfig(),
		raceEvent.GetEntryList().ToACServerConfig(),
		checksums,
		logger,
		acserver.MultiPlugin(activePlugins...),
	)

	if err != nil {
		return err
	}

	go func() {
		sp.run <- sp.server.Run()
	}()

	// @TODO embed content manager into the same HTTP server?
	if serverOptions.EnableContentManagerWrapper == 1 && serverOptions.ContentManagerWrapperPort > 0 {
		go panicCapture(func() {
			err := sp.contentManagerWrapper.Start(serverOptions.ContentManagerWrapperPort, sp.raceEvent, sp)

			if err != nil {
				logrus.WithError(err).Error("Could not start Content Manager wrapper server")
			}
		})
	}

	strackerOptions, err := sp.store.LoadStrackerOptions()
	strackerEnabled := err == nil && strackerOptions.EnableStracker && IsStrackerInstalled()

	kissMyRankOptions, err := sp.store.LoadKissMyRankOptions()
	kissMyRankEnabled := err == nil && kissMyRankOptions.EnableKissMyRank && IsKissMyRankInstalled()

	realPenaltyOptions, err := sp.store.LoadRealPenaltyOptions()
	realPenaltyEnabled := err == nil && realPenaltyOptions.RealPenaltyAppConfig.General.EnableRealPenalty && IsRealPenaltyInstalled()

	if (strackerEnabled || kissMyRankEnabled) && !udpPluginPortsSetup {
		logrus.WithError(ErrPluginConfigurationRequiresUDPPortSetup).Error("Please check your server configuration")
	}

	if strackerEnabled && strackerOptions != nil && udpPluginPortsSetup {
		strackerOptions.InstanceConfiguration.ACServerConfigIni = filepath.Join(ServerInstallPath, "cfg", serverConfigIniPath)
		strackerOptions.InstanceConfiguration.ACServerWorkingDir = ServerInstallPath
		strackerOptions.ACPlugin.SendPort = serverOptions.UDPPluginLocalPort
		strackerOptions.ACPlugin.ReceivePort = formValueAsInt(strings.Split(serverOptions.UDPPluginAddress, ":")[1])

		if kissMyRankEnabled || realPenaltyEnabled {
			// kissmyrank and real penalty use stracker's forwarding to chain the plugins. make sure that it is set up.
			if strackerOptions.ACPlugin.ProxyPluginLocalPort <= 0 {
				strackerOptions.ACPlugin.ProxyPluginLocalPort, err = FreeUDPPort()

				if err != nil {
					return err
				}
			}

			for strackerOptions.ACPlugin.ProxyPluginPort <= 0 || strackerOptions.ACPlugin.ProxyPluginPort == strackerOptions.ACPlugin.ProxyPluginLocalPort {
				strackerOptions.ACPlugin.ProxyPluginPort, err = FreeUDPPort()

				if err != nil {
					return err
				}
			}
		}

		if err := strackerOptions.Write(); err != nil {
			return err
		}

		err = sp.startPlugin(wd, &CommandPlugin{
			Executable: StrackerExecutablePath(),
			Arguments: []string{
				"--stracker_ini",
				filepath.Join(StrackerFolderPath(), strackerConfigIniFilename),
			},
		})

		if err != nil {
			return err
		}

		logrus.Infof("Started sTracker. Listening for pTracker connections on port %d", strackerOptions.InstanceConfiguration.ListeningPort)
	}

	if realPenaltyEnabled && realPenaltyOptions != nil && udpPluginPortsSetup {
		if err := fixRealPenaltyExecutablePermissions(); err != nil {
			return err
		}

		var (
			port     int
			response string
		)

		if !strackerEnabled {
			// connect to the forwarding address
			port, err = strconv.Atoi(strings.Split(serverOptions.UDPPluginAddress, ":")[1])

			if err != nil {
				return err
			}

			response = fmt.Sprintf("127.0.0.1:%d", serverOptions.UDPPluginLocalPort)
		} else {
			logrus.Infof("sTracker and Real Penalty both enabled. Using plugin forwarding method: [Server Manager] <-> [sTracker] <-> [Real Penalty]")

			// connect to stracker's proxy port
			port = strackerOptions.ACPlugin.ProxyPluginPort
			response = fmt.Sprintf("127.0.0.1:%d", strackerOptions.ACPlugin.ProxyPluginLocalPort)
		}

		if kissMyRankEnabled {
			// proxy from real penalty to kmr
			freeUDPPort, err := FreeUDPPort()

			if err != nil {
				return err
			}

			realPenaltyOptions.RealPenaltyAppConfig.PluginsRelay.UDPPort = strconv.Itoa(freeUDPPort)

			pluginPort, err := FreeUDPPort()

			if err != nil {
				return err
			}

			realPenaltyOptions.RealPenaltyAppConfig.PluginsRelay.OtherUDPPlugin = fmt.Sprintf("127.0.0.1:%d", pluginPort)
		}

		realPenaltyOptions.RealPenaltyAppConfig.General.UDPPort = port
		realPenaltyOptions.RealPenaltyAppConfig.General.UDPResponse = response
		realPenaltyOptions.RealPenaltyAppConfig.General.ACServerPath = ServerInstallPath
		realPenaltyOptions.RealPenaltyAppConfig.General.ACCFGFile = filepath.Join(ServerInstallPath, "cfg", "server_cfg.ini")
		realPenaltyOptions.RealPenaltyAppConfig.General.ACTracksFolder = filepath.Join(ServerInstallPath, "content", "tracks")
		realPenaltyOptions.RealPenaltyAppConfig.General.ACWeatherFolder = filepath.Join(ServerInstallPath, "content", "weather")
		realPenaltyOptions.RealPenaltyAppConfig.General.AppFile = filepath.Join(RealPenaltyFolderPath(), "files", "app")
		realPenaltyOptions.RealPenaltyAppConfig.General.ImagesFile = filepath.Join(RealPenaltyFolderPath(), "files", "images")
		realPenaltyOptions.RealPenaltyAppConfig.General.SoundsFile = filepath.Join(RealPenaltyFolderPath(), "files", "sounds")
		realPenaltyOptions.RealPenaltyAppConfig.General.TracksFolder = filepath.Join(RealPenaltyFolderPath(), "tracks")

		if err := realPenaltyOptions.Write(); err != nil {
			return err
		}

		err = sp.startPlugin(wd, &CommandPlugin{
			Executable: RealPenaltyExecutablePath(),
			Arguments: []string{
				"--print_on",
			},
		})

		if err != nil {
			return err
		}

		logrus.Infof("Started Real Penalty")
	}

	if kissMyRankEnabled && kissMyRankOptions != nil && udpPluginPortsSetup {
		if err := fixKissMyRankExecutablePermissions(); err != nil {
			return err
		}

		kissMyRankOptions.ACServerIP = "127.0.0.1"
		kissMyRankOptions.ACServerHTTPPort = serverOptions.HTTPPort
		kissMyRankOptions.UpdateInterval = config.LiveMap.IntervalMs
		kissMyRankOptions.ACServerResultsBasePath = ServerInstallPath

		raceConfig := sp.raceEvent.GetRaceConfig()
		entryList := sp.raceEvent.GetEntryList()

		kissMyRankOptions.MaxPlayers = raceConfig.MaxClients

		if len(entryList) > kissMyRankOptions.MaxPlayers {
			kissMyRankOptions.MaxPlayers = len(entryList)
		}

		if realPenaltyEnabled && realPenaltyOptions != nil {
			// realPenalty is enabled, use its relay port
			logrus.Infof("Real Penalty and KissMyRank both enabled. Using plugin forwarding method: [Previous Plugin/Server Manager] <-> [Real Penalty] <-> [KissMyRank]")

			kissMyRankOptions.ACServerPluginLocalPort = formValueAsInt(realPenaltyOptions.RealPenaltyAppConfig.PluginsRelay.UDPPort)
			kissMyRankOptions.ACServerPluginAddressPort = formValueAsInt(strings.Split(realPenaltyOptions.RealPenaltyAppConfig.PluginsRelay.OtherUDPPlugin, ":")[1])
		} else if strackerEnabled {
			// stracker is enabled, use its forwarding port
			logrus.Infof("sTracker and KissMyRank both enabled. Using plugin forwarding method: [Server Manager] <-> [sTracker] <-> [KissMyRank]")
			kissMyRankOptions.ACServerPluginLocalPort = strackerOptions.ACPlugin.ProxyPluginLocalPort
			kissMyRankOptions.ACServerPluginAddressPort = strackerOptions.ACPlugin.ProxyPluginPort
		} else {
			// stracker and real penalty are disabled, use our forwarding port
			kissMyRankOptions.ACServerPluginLocalPort = serverOptions.UDPPluginLocalPort
			kissMyRankOptions.ACServerPluginAddressPort = formValueAsInt(strings.Split(serverOptions.UDPPluginAddress, ":")[1])
		}

		if err := kissMyRankOptions.Write(); err != nil {
			return err
		}

		err = sp.startPlugin(wd, &CommandPlugin{
			Executable: KissMyRankExecutablePath(),
		})

		if err != nil {
			return err
		}

		logrus.Infof("Started KissMyRank")
	}

	for _, plugin := range config.Server.Plugins {
		err = sp.startPlugin(wd, plugin)

		if err != nil {
			logrus.WithError(err).Errorf("Could not run extra command: %s", plugin.String())
		}
	}

	return nil
}

func (sp *AssettoServerProcess) deleteOldLogFiles(numFilesToKeep int) error {
	if numFilesToKeep <= 0 {
		return nil
	}

	tidyFunc := func(directory string) error {
		logFiles, err := ioutil.ReadDir(directory)

		if err != nil {
			return err
		}

		if len(logFiles) >= numFilesToKeep {
			sort.Slice(logFiles, func(i, j int) bool {
				return logFiles[i].ModTime().After(logFiles[j].ModTime())
			})

			for _, f := range logFiles[numFilesToKeep-1:] {
				if err := os.Remove(filepath.Join(directory, f.Name())); err != nil {
					return err
				}
			}

			logrus.Debugf("Successfully cleared %d log files from %s", len(logFiles[numFilesToKeep-1:]), directory)
		}

		return nil
	}

	logDirectory := filepath.Join(ServerInstallPath, "logs", "session")

	if err := tidyFunc(logDirectory); err != nil {
		return err
	}

	return nil
}

func (sp *AssettoServerProcess) onStop() error {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()
	logrus.Debugf("Server stopped. Stopping UDP listener and child processes.")

	sp.raceEvent = nil

	sp.stopChildProcesses()

	for _, doneCh := range sp.notifyDoneChs {
		select {
		case doneCh <- struct{}{}:
		default:
		}
	}

	if sp.logFile != nil {
		if err := sp.logFile.Close(); err != nil {
			return err
		}

		sp.logFile = nil
	}

	return nil
}

func (sp *AssettoServerProcess) Logs() string {
	return sp.logBuffer.String()
}

func (sp *AssettoServerProcess) SharedPitLane() *pitlanedetection.PitLane {
	return sp.sharedPitLane
}

func (sp *AssettoServerProcess) Event() RaceEvent {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	if sp.raceEvent == nil {
		return QuickRace{}
	}

	return sp.raceEvent
}

func (sp *AssettoServerProcess) CurrentServerConfig() *GlobalServerConfig {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	return sp.serverConfig
}

func (sp *AssettoServerProcess) NotifyDone(ch chan struct{}) {
	sp.mutex.Lock()
	defer sp.mutex.Unlock()

	sp.notifyDoneChs = append(sp.notifyDoneChs, ch)
}

func (sp *AssettoServerProcess) startPlugin(wd string, plugin *CommandPlugin) error {
	commandFullPath, err := filepath.Abs(plugin.Executable)

	if err != nil {
		return err
	}

	ctx := context.Background()

	cmd := buildCommand(ctx, commandFullPath, plugin.Arguments...)

	pluginDir, err := filepath.Abs(filepath.Dir(commandFullPath))

	if err != nil {
		logrus.WithError(err).Warnf("Could not determine plugin directory. Setting working dir to: %s", wd)
		pluginDir = wd
	}

	cmd.Stdout = pluginsOutput
	cmd.Stderr = pluginsOutput

	cmd.Dir = pluginDir

	err = cmd.Start()

	if err != nil {
		return err
	}

	sp.extraProcesses = append(sp.extraProcesses, cmd)

	return nil
}

func (sp *AssettoServerProcess) stopChildProcesses() {
	sp.contentManagerWrapper.Stop()

	for _, command := range sp.extraProcesses {
		err := kill(command.Process)

		if err != nil {
			logrus.WithError(err).Errorf("Can't kill process: %d", command.Process.Pid)
			continue
		}

		_ = command.Process.Release()
	}

	sp.extraProcesses = make([]*exec.Cmd, 0)
}

func newLogBuffer(maxSize int) *logBuffer {
	return &logBuffer{
		size: maxSize,
		buf:  new(bytes.Buffer),
	}
}

type logBuffer struct {
	buf *bytes.Buffer

	size int

	mutex sync.Mutex
}

func (lb *logBuffer) Write(p []byte) (n int, err error) {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	b := lb.buf.Bytes()

	if len(b) > lb.size {
		lb.buf = bytes.NewBuffer(b[len(b)-lb.size:])
	}

	return lb.buf.Write(p)
}

func (lb *logBuffer) String() string {
	lb.mutex.Lock()
	defer lb.mutex.Unlock()

	return strings.Replace(lb.buf.String(), "\n\n", "\n", -1)
}

func FreeUDPPort() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", "localhost:0")

	if err != nil {
		return 0, err
	}

	l, err := net.ListenUDP("udp", addr)

	if err != nil {
		return 0, err
	}

	defer l.Close()

	return l.LocalAddr().(*net.UDPAddr).Port, nil
}

// noErrClosedWriter masks a write to not report the os.ErrClosed error.
type noErrClosedWriter struct {
	w io.Writer
}

func (nec *noErrClosedWriter) Write(p []byte) (n int, err error) {
	n, err = nec.w.Write(p)

	if errors.Is(err, os.ErrClosed) {
		return len(p), nil
	}

	return n, err
}
