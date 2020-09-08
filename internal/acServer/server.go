package acServer

import (
	"context"
	"runtime"
	"time"
)

type Server struct {
	state  *ServerState
	lobby  *Lobby
	plugin Plugin

	sessionManager      *SessionManager
	adminCommandManager *AdminCommandManager
	entryListManager    *EntryListManager

	tcp  *TCP
	udp  *UDP
	http *HTTP

	cfn context.CancelFunc
	ctx context.Context

	logger Logger

	stopped chan error
}

func NewServer(serverConfig *ServerConfig, raceConfig *RaceConfig, entryList EntryList, checksums []CustomChecksumFile, logger Logger, plugin Plugin) (*Server, error) {
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

	state, err := NewServerState(serverConfig, raceConfig, entryList, checksums, plugin, logger)

	if err != nil {
		return nil, err
	}

	lobby := NewLobby(state, logger)

	ctx, cfn := context.WithCancel(context.Background())

	server := &Server{
		state:   state,
		lobby:   lobby,
		plugin:  plugin,
		stopped: make(chan error, 1),
		ctx:     ctx,
		cfn:     cfn,
		logger:  logger,
	}

	server.sessionManager = NewSessionManager(state, lobby, plugin, logger, server.Stop)
	server.adminCommandManager = NewAdminCommandManager(state, server.sessionManager, logger)
	server.entryListManager = NewEntryListManager(state, logger)

	if err := plugin.Init(server, logger); err != nil {
		return server, err
	}

	return server, nil
}

func (s *Server) Start() error {
	runtime.GOMAXPROCS(s.state.serverConfig.NumberOfThreads)
	s.logger.Infof("Initialising openAcServer with compatibility for server version %d", CurrentProtocolVersion)

	s.tcp = NewTCP(s.state.serverConfig.TCPPort, s)
	s.udp = NewUDP(s.state.serverConfig.UDPPort, s)
	s.http = NewHTTP(s.state.serverConfig.HTTPPort, s.state, s.sessionManager, s.entryListManager, s.logger)

	err := s.tcp.Listen()

	if err != nil {
		return err
	}

	s.state.packetConn, err = s.udp.Listen()

	if err != nil {
		return err
	}

	go func() {
		err := s.plugin.OnVersion(CurrentProtocolVersion)

		if err != nil {
			s.logger.WithError(err).Error("On version plugin returned an error")
		}
	}()

	go s.loop()

	err = s.http.Listen()

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

	if err = s.tcp.Close(); err != nil {
		return err
	}

	if err = s.udp.Close(); err != nil {
		return err
	}

	if err = s.http.Close(); err != nil {
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
	lastSunUpdate := int64(0)

	// @TODO what is the performance impact of this? Turn off when CSP/Sol enabled (probably)

	sunAngleUpdateInterval := int64(60000)

	if s.state.raceConfig.TimeOfDayMultiplier > 0 {
		sunAngleUpdateInterval = int64(float32(60000) / float32(s.state.raceConfig.TimeOfDayMultiplier))
	}

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

						go func(carCopy *Car) {
							err := s.plugin.OnCarUpdate(*carCopy)

							if err != nil {
								s.logger.WithError(err).Error("On car update plugin returned an error")
							}
						}(car)

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

						if err := px.WriteUDP(s.state.packetConn, car.Connection.udpAddr); err != nil {
							s.logger.WithError(err).Error("Could not write ping")
						}
					}

					car.UpdatePriorities(s.state.entryList)
				}
			}

			// update sun angle
			// @TODO (improvement) at 1x this loses between 0.5 and 1s evey 60s
			if (sleepTime != idleSleepTime) && currentTime-lastSunUpdate > sunAngleUpdateInterval || lastSunUpdate == 0 {
				// @TODO with CSP exceeding -80 and 80 works fine, and you can loop!
				s.state.sunAngle = s.state.raceConfig.SunAngle + float32(s.state.raceConfig.TimeOfDayMultiplier)*(0.0044*(float32(currentTime)/1000.0))

				if s.state.sunAngle < -80 {
					s.state.sunAngle = -80
				}

				if s.state.sunAngle > 80 {
					s.state.sunAngle = 80
				}

				s.state.SendSunAngle()

				lastSunUpdate = currentTime
			}

			// update weather
			if s.sessionManager.weatherProgression && (sleepTime != idleSleepTime) && s.sessionManager.nextWeatherUpdate < currentTime {
				s.sessionManager.NextWeather(currentTime)
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
		AmbientTemp:     s.state.currentWeather.Ambient,
		RoadTemp:        s.state.currentWeather.Road,
		WeatherGraphics: s.state.currentWeather.GraphicsName,
		ElapsedTime:     s.sessionManager.ElapsedSessionTime(),
		SessionType:     s.state.currentSession.SessionType,
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
