package plugins

import (
	"fmt"
	"net"
	"time"

	"justapengu.in/acsm/internal/acserver"
)

type UDPPluginEvent uint8

const (
	// Send
	EventCollisionWithCar         UDPPluginEvent = 10
	EventCollisionWithEnvironment UDPPluginEvent = 11
	EventNewSession               UDPPluginEvent = 50
	EventNewConnection            UDPPluginEvent = 51
	EventConnectionClosed         UDPPluginEvent = 52
	EventCarUpdate                UDPPluginEvent = 53
	EventCarInfo                  UDPPluginEvent = 54
	EventEndSession               UDPPluginEvent = 55
	EventVersion                  UDPPluginEvent = 56
	EventChat                     UDPPluginEvent = 57
	EventClientLoaded             UDPPluginEvent = 58
	EventSessionInfo              UDPPluginEvent = 59
	EventLapCompleted             UDPPluginEvent = 73
	EventClientEvent              UDPPluginEvent = 130
	EventSectorCompleted          UDPPluginEvent = 150

	// Receive
	EventRealTimePositionInterval UDPPluginEvent = 200
	EventGetCarInfo               UDPPluginEvent = 201
	EventSendChat                 UDPPluginEvent = 202
	EventBroadcastChat            UDPPluginEvent = 203
	EventGetSessionInfo           UDPPluginEvent = 204
	EventSetSessionInfo           UDPPluginEvent = 205
	EventKickUser                 UDPPluginEvent = 206
	EventNextSession              UDPPluginEvent = 207
	EventRestartSession           UDPPluginEvent = 208
	EventAdminCommand             UDPPluginEvent = 209
	EventEnableEnhancedReporting  UDPPluginEvent = 210
)

type UDPPlugin struct {
	localAddress  *net.UDPAddr
	remoteAddress *net.UDPAddr
	packetConn    *net.UDPConn

	server acserver.ServerPlugin
	logger acserver.Logger

	enableEnhancedReporting bool
}

func NewUDPPlugin(listenPort int, sendAddress string) (*UDPPlugin, error) {
	remoteAddress, err := net.ResolveUDPAddr("udp", sendAddress)

	if err != nil {
		return nil, err
	}

	localAddress, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", listenPort))

	if err != nil {
		return nil, err
	}

	p := &UDPPlugin{
		localAddress:  localAddress,
		remoteAddress: remoteAddress,
	}

	return p, nil
}

func (u *UDPPlugin) listen() {
	for {
		buf := make([]byte, 1024)

		n, _, err := u.packetConn.ReadFrom(buf)

		if err != nil {
			u.logger.WithError(err).Error("udp plugin: could not read from udp buffer")
			continue
		}

		if err := u.handleConnection(buf[:n]); err != nil {
			u.logger.WithError(err).Error("udp plugin: could not handle udp connection")
			return
		}
	}
}

func (u *UDPPlugin) Init(server acserver.ServerPlugin, logger acserver.Logger) error {
	u.server = server
	u.logger = logger

	var err error

	u.packetConn, err = net.DialUDP("udp", u.localAddress, u.remoteAddress)

	if err != nil {
		return err
	}

	go u.listen()

	return nil
}

func (u *UDPPlugin) handleConnection(data []byte) error {
	p := acserver.NewPacket(data)

	var messageType UDPPluginEvent

	p.Read(&messageType)

	switch messageType {
	case EventRealTimePositionInterval:
		interval := p.ReadUint16()
		u.server.SetUpdateInterval(time.Millisecond * time.Duration(interval))
	case EventGetCarInfo:
		var carID acserver.CarID

		p.Read(&carID)

		car, err := u.server.GetCarInfo(carID)

		if err != nil {
			return err
		}

		response := carInfoPacket(EventCarInfo, car)
		return response.WriteToUDPConn(u.packetConn)
	case EventSendChat:
		var carID acserver.CarID

		p.Read(&carID)

		return u.server.SendChat(p.ReadUTF32String(), acserver.ServerCarID, carID)
	case EventBroadcastChat:
		u.server.BroadcastChat(p.ReadUTF32String(), acserver.ServerCarID)
	case EventGetSessionInfo:
		response := sessionInfoPacket(EventSessionInfo, u.server.GetSessionInfo())

		return response.WriteToUDPConn(u.packetConn)
	case EventKickUser:
		var carID acserver.CarID

		p.Read(&carID)

		return u.server.KickUser(carID, acserver.KickReasonGeneric)
	case EventNextSession:
		u.server.NextSession()
	case EventRestartSession:
		u.server.RestartSession()
	case EventAdminCommand:
		return u.server.AdminCommand(p.ReadUTF32String())
	case EventSetSessionInfo:
		var sessionIndex uint8

		p.Read(&sessionIndex)

		name := p.ReadUTF32String()

		var sessionType acserver.SessionType

		p.Read(&sessionType)

		laps := p.ReadUint32()
		length := p.ReadUint32()
		waitTime := p.ReadUint32()

		session := &acserver.SessionConfig{
			SessionType: sessionType,
			Name:        name,
			Time:        uint16(length),
			Laps:        uint16(laps),
			IsOpen:      acserver.FreeJoin,
			WaitTime:    int(waitTime) * 1000,
		}

		u.server.SetCurrentSession(sessionIndex, session)

		return nil
	case EventEnableEnhancedReporting:
		u.enableEnhancedReporting = true
	default:
		return fmt.Errorf("unknown message type: %d", messageType)
	}

	return nil
}

func carInfoPacket(messageType UDPPluginEvent, car acserver.Car) *acserver.Packet {
	p := acserver.NewPacket(nil)
	p.Write(messageType)
	p.WriteUTF32String(car.Driver.Name)
	p.WriteUTF32String(car.Driver.GUID)
	p.Write(car.CarID)
	p.WriteString(car.Model)
	p.WriteString(car.Skin)

	return p
}

func (u *UDPPlugin) OnNewConnection(car acserver.Car) error {
	p := carInfoPacket(EventNewConnection, car)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnConnectionClosed(car acserver.Car) error {
	p := carInfoPacket(EventConnectionClosed, car)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnCarUpdate(car acserver.Car) error {
	p := acserver.NewPacket(nil)
	p.Write(EventCarUpdate)
	p.Write(car.CarID)
	p.Write(car.Status.Position)
	p.Write(car.Status.Velocity)
	p.Write(car.Status.GearIndex)
	p.Write(car.Status.EngineRPM)
	p.Write(car.Status.NormalisedSplinePos)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnNewSession(newSession acserver.SessionInfo) error {
	p := sessionInfoPacket(EventNewSession, newSession)

	return p.WriteToUDPConn(u.packetConn)
}

func sessionInfoPacket(eventType UDPPluginEvent, sessionInfo acserver.SessionInfo) *acserver.Packet {
	p := acserver.NewPacket(nil)
	p.Write(eventType)
	p.Write(uint8(acserver.CurrentProtocolVersion))
	p.Write(sessionInfo.SessionIndex)
	p.Write(sessionInfo.SessionIndex) // @TODO this one should be 'current session index'?
	p.Write(sessionInfo.SessionCount)
	p.WriteUTF32String(sessionInfo.ServerName)
	p.WriteString(sessionInfo.Track)
	p.WriteString(sessionInfo.TrackConfig)
	p.WriteString(sessionInfo.Name)
	p.Write(sessionInfo.SessionType)
	p.Write(sessionInfo.NumMinutes)
	p.Write(sessionInfo.NumLaps)
	p.Write(uint16(sessionInfo.WaitTime))
	p.Write(sessionInfo.AmbientTemp)
	p.Write(sessionInfo.RoadTemp)
	p.WriteString(sessionInfo.WeatherGraphics)
	p.Write(int32(sessionInfo.ElapsedTime.Milliseconds()))

	return p
}

func (u *UDPPlugin) OnEndSession(sessionFile string) error {
	p := acserver.NewPacket(nil)
	p.Write(EventEndSession)
	p.WriteUTF32String(sessionFile)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnVersion(version uint16) error {
	p := acserver.NewPacket(nil)
	p.Write(EventVersion)
	p.Write(uint8(version))

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnChat(chat acserver.Chat) error {
	p := acserver.NewPacket(nil)
	p.Write(EventChat)
	p.Write(chat.FromCar)
	p.WriteUTF32String(chat.Message)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnClientLoaded(car acserver.Car) error {
	p := acserver.NewPacket(nil)
	p.Write(EventClientLoaded)
	p.Write(car.CarID)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {
	p := acserver.NewPacket(nil)
	p.Write(EventLapCompleted)
	p.Write(carID)
	p.Write(uint32(lap.LapTime.Milliseconds()))
	p.Write(uint8(lap.Cuts))

	leaderboard := u.server.GetLeaderboard()
	p.Write(uint8(len(leaderboard)))

	for _, line := range leaderboard {
		p.Write(line.Car.CarID)
		p.Write(uint32(line.Time.Milliseconds()))
		p.Write(uint16(line.Car.SessionData.LapCount))
		if line.Car.SessionData.HasCompletedSession {
			p.Write(uint8(1))
		} else {
			p.Write(uint8(0))
		}
	}

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnClientEvent(_ acserver.ClientEvent) error {
	return nil
}

func (u *UDPPlugin) OnCollisionWithCar(event acserver.ClientEvent) error {
	p := acserver.NewPacket(nil)
	p.Write(EventClientEvent)
	p.Write(EventCollisionWithCar)
	p.Write(event.CarID)
	p.Write(event.OtherCarID)
	p.Write(event.Speed)
	p.Write(event.Position)
	p.Write(event.RelativePosition)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnCollisionWithEnv(event acserver.ClientEvent) error {
	p := acserver.NewPacket(nil)
	p.Write(EventClientEvent)
	p.Write(EventCollisionWithEnvironment)
	p.Write(event.CarID)
	p.Write(event.Speed)
	p.Write(event.Position)
	p.Write(event.RelativePosition)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnSectorCompleted(split acserver.Split) error {
	if !u.enableEnhancedReporting {
		return nil
	}

	p := acserver.NewPacket(nil)
	p.Write(EventSectorCompleted)
	p.Write(split.Car.CarID)
	p.Write(split.Index)
	p.Write(split.Time)
	p.Write(split.Cuts)

	return p.WriteToUDPConn(u.packetConn)
}

func (u *UDPPlugin) OnWeatherChange(_ acserver.CurrentWeather) error {
	p := sessionInfoPacket(EventSessionInfo, u.server.GetSessionInfo())

	return p.WriteToUDPConn(u.packetConn)
}
