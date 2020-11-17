package acserver

import (
	"context"
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
	if plugin == nil {
		plugin = nilPlugin{}
	}

	if len(entryList) > raceConfig.MaxClients {
		logger.Warnf("Entry List length exceeds configured MaxClients value. Increasing to match.")
		raceConfig.MaxClients = len(entryList)
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

	return server, nil
}

func (s *Server) Start() error {
	runtime.GOMAXPROCS(s.state.serverConfig.NumberOfThreads)
	s.logger.Infof("Initialising acServer with compatibility for server version %d", CurrentProtocolVersion)

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

						if err := s.state.DisconnectCar(car); err != nil {
							s.logger.WithError(err).Errorf("Could not broadcast timed out car disconnect")
						}

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
				s.logger.Errorf("CPU overload detected! Previous update took %dms", totalTimeForUpdate)
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

func (s *Server) GetSessionInfo() SessionInfo {
	currentWeather := s.weatherManager.GetCurrentWeather()
	currentSession := s.sessionManager.GetCurrentSession()

	return SessionInfo{
		Version:         CurrentResultsVersion,
		SessionIndex:    s.sessionManager.GetSessionIndex(),
		SessionCount:    uint8(len(s.state.raceConfig.Sessions)),
		ServerName:      s.state.serverConfig.Name,
		Track:           s.state.raceConfig.Track,
		TrackConfig:     s.state.raceConfig.TrackLayout,
		Name:            currentSession.Name,
		NumMinutes:      currentSession.Time,
		NumLaps:         currentSession.Laps,
		WaitTime:        currentSession.WaitTime,
		AmbientTemp:     currentWeather.Ambient,
		RoadTemp:        currentWeather.Road,
		WeatherGraphics: currentWeather.GraphicsName,
		ElapsedTime:     s.sessionManager.ElapsedSessionTime(),
		SessionType:     currentSession.SessionType,
		IsSolo:          currentSession.Solo,
	}
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
					s.logger.WithError(err).Errorf("Could not send car update for car: %d", car.CarID)
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
