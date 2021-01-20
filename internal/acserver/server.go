package acserver

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

type Server struct {
	state         *ServerState
	lobby         *Lobby
	plugin        Plugin
	baseDirectory string

	sessionManager      *SessionManager
	adminCommandManager *AdminCommandManager
	entryListManager    *EntryListManager
	weatherManager      *WeatherManager
	checksumManager     *ChecksumManager
	dynamicTrack        *DynamicTrack

	tcp  *TCP
	udp  *UDP
	http *HTTP

	cfn context.CancelFunc
	ctx context.Context

	logger Logger

	tcpError, udpError, stopped chan error
	pluginUpdateInterval        chan time.Duration
	carUpdateOnce               sync.Once
}

func NewServer(ctx context.Context, baseDirectory string, serverConfig *ServerConfig, raceConfig *EventConfig, entryList EntryList, checksums []CustomChecksumFile, logger Logger, plugin Plugin) (*Server, error) {
	logger.Infof("Initialising acServer with compatibility for server version %d", CurrentProtocolVersion)

	if plugin == nil {
		plugin = nilPlugin{}
	}

	if len(entryList) > raceConfig.MaxClients {
		logger.Warnf("Entry List length exceeds configured MaxClients value. Increasing to match.")
		raceConfig.MaxClients = len(entryList)
	}

	for _, session := range raceConfig.Sessions {
		if session.SessionType == SessionTypeRace && session.Time == 0 && session.Laps <= 3 {
			logger.Infof("Race session has less than 3 laps. Enabling teleport penalty")
			raceConfig.StartRule = StartRuleTeleportToPits
			break
		}
	}

	if raceConfig.HasSession(SessionTypeBooking) {
		raceConfig.PickupModeEnabled = false
	}

	dynamicTrack := NewDynamicTrack(logger, raceConfig.DynamicTrack)

	state, err := NewServerState(baseDirectory, serverConfig, raceConfig, entryList, plugin, logger, dynamicTrack)

	if err != nil {
		return nil, err
	}

	ctx, cfn := context.WithCancel(ctx)
	lobby := NewLobby(ctx, state, logger)

	server := &Server{
		state:                state,
		lobby:                lobby,
		plugin:               plugin,
		stopped:              make(chan error, 1),
		tcpError:             make(chan error, 1),
		udpError:             make(chan error, 1),
		ctx:                  ctx,
		cfn:                  cfn,
		logger:               logger,
		baseDirectory:        baseDirectory,
		pluginUpdateInterval: make(chan time.Duration),
	}

	server.checksumManager, err = NewChecksumManager(baseDirectory, state, logger, checksums)

	if err != nil {
		return nil, err
	}

	server.weatherManager = NewWeatherManager(state, plugin, logger)
	server.sessionManager = NewSessionManager(state, server.weatherManager, lobby, dynamicTrack, plugin, logger, server.Stop, baseDirectory)
	server.adminCommandManager = NewAdminCommandManager(state, server.sessionManager, server.weatherManager, logger)
	server.entryListManager = NewEntryListManager(state, logger)
	server.dynamicTrack = dynamicTrack

	clearStatistics()

	return server, nil
}

func (s *Server) Start() error {
	runtime.GOMAXPROCS(s.state.serverConfig.NumberOfThreads)

	s.tcp = NewTCP(s.state.serverConfig.TCPPort, s)
	s.udp = NewUDP(s.state.serverConfig.UDPPort, s, s.state.serverConfig.ReceiveBufferSize, s.state.serverConfig.SendBufferSize)
	s.http = NewHTTP(s.state.serverConfig.HTTPPort, s.state, s.sessionManager, s.entryListManager, s.logger)

	go func() {
		s.tcpError <- s.tcp.Listen(s.ctx)
	}()

	go func() {
		s.udpError <- s.udp.Listen(s.ctx)
	}()

	if err := s.plugin.Init(s, s.logger); err != nil {
		return err
	}

	s.state.udp = s.udp

	err := s.plugin.OnVersion(CurrentProtocolVersion)

	if err != nil {
		s.logger.WithError(err).Error("On version plugin returned an error")
	}

	go s.loop()

	err = s.http.Listen()

	if err != nil {
		return err
	}

	if s.state.serverConfig.RegisterToLobby {
		if err := s.lobby.Try("Register to lobby", s.lobby.Register, true); err != nil {
			s.logger.WithError(err).Error("All attempts to register to lobby failed")
			return s.Stop(false)
		}
	}

	go s.sessionManager.loop(s.ctx)

	return nil
}

func (s *Server) Stop(persistResults bool) (err error) {
	defer func() {
		s.stopped <- err
	}()

	if persistResults {
		s.sessionManager.SaveResultsAndBuildLeaderboard(false)
	}

	s.logger.Infof("Shutting down acServer")

	if err := s.plugin.Shutdown(); err != nil {
		s.logger.WithError(err).Errorf("Plugin shutdown reported error")
	}

	s.cfn()

	if err = s.http.Close(); err != nil {
		return err
	}

	if err := <-s.tcpError; err != nil {
		return err
	}

	if err := <-s.udpError; err != nil {
		return err
	}

	s.state.Close()

	printStatistics(s.logger)

	return nil
}

func (s *Server) Run() error {
	if err := s.Start(); err != nil {
		return err
	}

	return <-s.stopped
}

const connectionTimeout = time.Minute

func (s *Server) loop() {
	lastSendTime := int64(0)

	if s.state.serverConfig.SleepTime < 1 {
		s.state.serverConfig.SleepTime = 1
	}

	s.sessionManager.NextSession(false, false)

	activeSleepTime := time.Millisecond * time.Duration(s.state.serverConfig.SleepTime)
	sleepTime := activeSleepTime

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debugf("Stopping main server loop")
			return
		default:
			currentTime := s.state.CurrentTimeMillisecond()
			isServerTick := currentTime-lastSendTime >= serverTickRate

			for _, car := range s.state.entryList {
				if !car.IsConnected() || !car.HasSentFirstUpdate() {
					continue
				}

				if car.HasUpdateToBroadcast() {
					s.state.BroadcastCarUpdate(car)

					car.SetHasUpdateToBroadcast(false)
				}

				car.UpdatePriorities(s.state.entryList)

				if isServerTick {
					if err := s.state.SendStatus(car, currentTime); err != nil {
						s.logger.WithError(err).Error("Could not send car status")
					}

					if time.Since(car.GetLastUpdateReceivedTime()) > connectionTimeout {
						s.logger.Warnf("Car: '%s' has not been seen in %s. Disconnecting...", car.String(), connectionTimeout)

						s.state.DisconnectCar(car)

						continue
					}

					if time.Since(car.Connection.LastPingTime) > time.Second {
						px := NewPacket(nil)
						px.Write(UDPMessagePong)
						px.Write(uint32(currentTime))
						px.Write(uint16(car.Connection.Ping))
						car.Connection.LastPingTime = time.Now()

						if err := px.WriteUDP(s.udp, car.Connection.udpAddr); err != nil {
							s.logger.WithError(err).Error("Could not write ping")
						}
					}
				}
			}

			if isServerTick {
				s.weatherManager.Step(currentTime, s.sessionManager.GetCurrentSession())
				lastSendTime = currentTime
			}

			if s.state.entryList.NumConnected() == 0 {
				if sleepTime != idleSleepTime {
					s.logger.Infof("No cars connected. Switching to idle sleep mode")
					sleepTime = idleSleepTime
				}
			} else {
				if sleepTime == idleSleepTime {
					s.logger.Infof("Cars connected, waking from idle")
					sleepTime = activeSleepTime
				}
			}

			totalTimeForUpdate := s.state.CurrentTimeMillisecond() - currentTime

			if sleepTime != idleSleepTime && totalTimeForUpdate > serverTickRate {
				s.logger.Warnf("CPU overload detected! Previous update took %dms. Catching up now.", totalTimeForUpdate)
			} else {
				time.Sleep(sleepTime)
			}
		}
	}
}

func (s *Server) GetCarInfo(id CarID) (CarInfo, error) {
	car, err := s.state.GetCarByID(id)

	if err != nil {
		return CarInfo{}, err
	}

	return car.Copy(), nil
}

func (s *Server) CarIsConnected(id CarID) bool {
	car, err := s.state.GetCarByID(id)

	if err != nil {
		return false
	}

	return car.IsConnected()
}

func (s *Server) GetSessionInfo(sessionIndex int) SessionInfo {
	return s.sessionManager.BuildSessionInfo(sessionIndex)
}

func (s *Server) AddDriver(name, team, guid, model string) error {
	driver := Driver{
		Name:     name,
		Team:     team,
		GUID:     guid,
		JoinTime: 0,
		LoadTime: time.Time{},
		Nation:   "",
	}

	return s.entryListManager.AddDriverToEmptyCar(driver, model)
}

func (s *Server) GetEventConfig() EventConfig {
	return *s.state.raceConfig
}

func (s *Server) SendChat(message string, from, to CarID, rateLimit bool) error {
	return s.state.SendChat(from, to, message, rateLimit)
}

func (s *Server) BroadcastChat(message string, from CarID, rateLimit bool) {
	s.state.BroadcastChat(from, message, rateLimit)
}

func (s *Server) UpdateBoP(carIDToUpdate CarID, ballast, restrictor float32) error {
	car, err := s.state.GetCarByID(carIDToUpdate)

	if err != nil {
		return err
	}

	car.Ballast = ballast
	car.Restrictor = restrictor

	s.state.BroadcastUpdateBoP(car)

	return nil
}

func (s *Server) KickUser(carIDToKick CarID, reason KickReason) error {
	return s.state.Kick(carIDToKick, reason)
}

func (s *Server) NextSession() {
	s.sessionManager.NextSession(true, false)
}

func (s *Server) RestartSession() {
	s.sessionManager.RestartSession()
}

func (s *Server) SetCurrentSession(index uint8, config *SessionConfig) {
	if int(index) >= len(s.state.raceConfig.Sessions) {
		return
	}

	s.state.raceConfig.Sessions[index] = config

	s.sessionManager.SetSessionIndex(index)

	s.sessionManager.RestartSession()
}

func (s *Server) AdminCommand(command string) error {
	serverEntrant := &Car{
		CarInfo: CarInfo{
			Driver:  Driver{Name: "Server"},
			CarID:   ServerCarID,
			IsAdmin: true,
		},
	}

	return s.adminCommandManager.Command(serverEntrant, command)
}

func (s *Server) GetLeaderboard() []*LeaderboardLine {
	currentSession := s.sessionManager.GetCurrentSession()

	return s.state.Leaderboard(currentSession.SessionType)
}

func (s *Server) SendSetup(overrideValues map[string]float32, carID CarID) error {
	car, err := s.state.GetCarByID(carID)

	if err != nil {
		return err
	}

	if car.FixedSetup != "" {
		// modify existing setup
		if _, ok := s.state.setups[car.FixedSetup]; !ok {
			s.logger.Infof("Fixed setup %s was selected for %s, but was not found on event start! Setup not applied!", car.FixedSetup, car.Driver.Name)
			return nil
		}

		for key, val := range overrideValues {
			s.state.setups[car.FixedSetup].values[key] = val
		}
	} else {
		// create a new setup
		car.FixedSetup = fmt.Sprintf("override-car-%d", car.CarID)

		s.state.setups[car.FixedSetup] = Setup{
			carName: car.Model,
			isFixed: 1,
			values:  overrideValues,
		}
	}

	s.state.SendSetup(car)

	return nil
}

func (s *Server) SetUpdateInterval(interval time.Duration) {
	s.carUpdateOnce.Do(func() {
		go s.pluginPositionUpdate()
	})

	s.pluginUpdateInterval <- interval
}

func (s *Server) pluginPositionUpdate() {
	interval := <-s.pluginUpdateInterval
	s.logger.Infof("Will send car updates at interval: %s", interval)
	ticker := time.NewTicker(interval)

	for {
		select {
		case <-ticker.C:
			for _, car := range s.state.entryList {
				if !car.IsConnected() || !car.HasSentFirstUpdate() {
					continue
				}

				if err := s.plugin.OnCarUpdate(car.Copy()); err != nil {
					s.logger.WithError(err).Debugf("Could not send car update for car: %d", car.CarID)
				}
			}
		case v := <-s.pluginUpdateInterval:
			s.logger.Infof("Updated to send car updates at interval: %s", interval)
			ticker.Reset(v)
		case <-s.ctx.Done():
			s.logger.Debugf("Stopped sending car updates")
			ticker.Stop()
			return
		}
	}
}
