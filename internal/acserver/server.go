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

	tcp  *TCP
	udp  *UDP
	http *HTTP

	cfn context.CancelFunc
	ctx context.Context

	logger Logger

	tcpError             chan error
	udpError             chan error
	stopped              chan error
	pluginUpdateInterval chan time.Duration
	carUpdateOnce        sync.Once
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

	state, err := NewServerState(baseDirectory, serverConfig, raceConfig, entryList, plugin, logger)

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
	server.sessionManager = NewSessionManager(state, server.weatherManager, lobby, plugin, logger, server.Stop, baseDirectory)
	server.adminCommandManager = NewAdminCommandManager(state, server.sessionManager, server.weatherManager, logger)
	server.entryListManager = NewEntryListManager(state, logger)

	return server, nil
}

func (s *Server) Start() error {
	runtime.GOMAXPROCS(s.state.serverConfig.NumberOfThreads)
	s.logger.Infof("Initialising openAcServer with compatibility for server version %d", CurrentProtocolVersion)

	s.tcp = NewTCP(s.state.serverConfig.TCPPort, s)
	s.udp = NewUDP(s.state.serverConfig.UDPPort, s)
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

	go func() {
		err := s.plugin.OnVersion(CurrentProtocolVersion)

		if err != nil {
			s.logger.WithError(err).Error("On version plugin returned an error")
		}
	}()

	go s.loop()

	err := s.http.Listen()

	if err != nil {
		return err
	}

	if s.state.serverConfig.RegisterToLobby {
		if err := s.lobby.Try("Register to lobby", s.lobby.Register); err != nil {
			s.logger.WithError(err).Error("All attempts to register to lobby failed")
			return s.Stop()
		}
	}

	go s.sessionManager.loop(s.ctx)

	return nil
}

func (s *Server) Stop() (err error) {
	defer func() {
		s.stopped <- err
	}()

	s.logger.Infof("Shutting down acServer")

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

	return nil
}

func (s *Server) Run() error {
	if err := s.Start(); err != nil {
		return err
	}

	return <-s.stopped
}

func (s *Server) loop() {
	lastSendTime := int64(0)

	if s.state.serverConfig.SleepTime < 1 {
		s.state.serverConfig.SleepTime = 1
	}

	s.sessionManager.NextSession(false)

	activeSleepTime := time.Millisecond * time.Duration(s.state.serverConfig.SleepTime)
	sleepTime := activeSleepTime

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Debugf("Stopping main server loop")
			return
		default:
			currentTime := currentTimeMillisecond()

			for _, car := range s.state.entryList {
				if car.IsConnected() && car.Connection.HasSentFirstUpdate {
					if car.HasUpdateToBroadcast {
						s.state.BroadcastCarUpdate(car)

						car.HasUpdateToBroadcast = false
					}

					if currentTime-lastSendTime >= serverTickRate {
						if err := s.state.SendStatus(car, currentTime); err != nil {
							s.logger.WithError(err).Error("Could not send car status")
						}
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

					car.UpdatePriorities(s.state.entryList)
				}
			}

			if sleepTime != idleSleepTime {
				s.weatherManager.Step(currentTime)
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

			lastSendTime = currentTime
			time.Sleep(sleepTime)
		}
	}
}

func (s *Server) GetCarInfo(id CarID) (Car, error) {
	car, err := s.state.GetCarByID(id)

	if err != nil {
		return Car{}, err
	}

	return *car, nil
}

func (s *Server) GetSessionInfo() SessionInfo {
	return SessionInfo{
		Version:         CurrentResultsVersion,
		SessionIndex:    s.state.currentSessionIndex,
		SessionCount:    uint8(len(s.state.raceConfig.Sessions)),
		ServerName:      s.state.serverConfig.Name,
		Track:           s.state.raceConfig.Track,
		TrackConfig:     s.state.raceConfig.TrackLayout,
		Name:            s.state.currentSession.Name,
		NumMinutes:      s.state.currentSession.Time,
		NumLaps:         s.state.currentSession.Laps,
		WaitTime:        s.state.currentSession.WaitTime,
		AmbientTemp:     s.weatherManager.currentWeather.Ambient,
		RoadTemp:        s.weatherManager.currentWeather.Road,
		WeatherGraphics: s.weatherManager.currentWeather.GraphicsName,
		ElapsedTime:     s.sessionManager.ElapsedSessionTime(),
		SessionType:     s.state.currentSession.SessionType,
		IsSolo:          s.state.currentSession.Solo,
	}
}

func (s *Server) SendChat(message string, from, to CarID) error {
	return s.state.SendChat(from, to, message)
}

func (s *Server) BroadcastChat(message string, from CarID) {
	s.state.BroadcastChat(from, message)
}

func (s *Server) KickUser(carIDToKick CarID, reason KickReason) error {
	return s.state.Kick(carIDToKick, reason)
}

func (s *Server) NextSession() {
	s.sessionManager.NextSession(true)
}

func (s *Server) RestartSession() {
	s.sessionManager.RestartSession()
}

func (s *Server) SetCurrentSession(index uint8, config *SessionConfig) {
	if int(index) >= len(s.state.raceConfig.Sessions) {
		return
	}

	s.state.raceConfig.Sessions[index] = config
	s.state.currentSessionIndex = index

	s.sessionManager.RestartSession()
}

func (s *Server) AdminCommand(command string) error {
	serverEntrant := &Car{
		Driver:  Driver{Name: "Server"},
		CarID:   ServerCarID,
		IsAdmin: true,
	}

	return s.adminCommandManager.Command(serverEntrant, command)
}

func (s *Server) GetLeaderboard() []*LeaderboardLine {
	return s.state.Leaderboard()
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
				if !car.IsConnected() || !car.Connection.HasSentFirstUpdate {
					continue
				}

				if err := s.plugin.OnCarUpdate(*car); err != nil {
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
