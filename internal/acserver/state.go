package acserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cj123/ini"
)

func init() {
	rand.Seed(time.Now().Unix())
}

const (
	serverTickRate = 48
	idleSleepTime  = time.Millisecond * 500
)

type ServerState struct {
	entryList    EntryList
	raceConfig   *EventConfig
	serverConfig *ServerConfig
	plugin       Plugin
	logger       Logger
	dynamicTrack *DynamicTrack

	serverStartTime time.Time

	udp           *UDP
	baseDirectory string

	// modifiable
	randomSeed uint32

	// fixed
	drsZones             map[string]DRSZone
	setups               map[string]Setup
	messageOfTheDay      string
	blockList            []string
	noJoinList           map[string]bool
	broadcastChatLimiter *time.Ticker
}

type Setup struct {
	carName string
	isFixed uint8 // this is ignored by the client, but hey ho it's here
	values  map[string]float32
}

func NewServerState(baseDirectory string, serverConfig *ServerConfig, raceConfig *EventConfig, entryList EntryList, plugin Plugin, logger Logger, dynamicTrack *DynamicTrack) (*ServerState, error) {
	ss := &ServerState{
		serverConfig:         serverConfig,
		raceConfig:           raceConfig,
		entryList:            entryList,
		plugin:               plugin,
		logger:               logger,
		dynamicTrack:         dynamicTrack,
		randomSeed:           rand.Uint32(),
		noJoinList:           make(map[string]bool),
		baseDirectory:        baseDirectory,
		broadcastChatLimiter: time.NewTicker(chatLimit),
		serverStartTime:      time.Now(),
	}

	if err := ss.init(); err != nil {
		return nil, err
	}

	return ss, nil
}

func (ss *ServerState) init() error {
	if err := ss.initDRSZones(); err != nil {
		ss.logger.WithError(err).Warnf("Could not load DRS zones, server will run without")
	}

	if err := ss.initFixedSetups(); err != nil {
		return err
	}

	if err := ss.initMOTD(); err != nil {
		return err
	}

	if err := ss.initBlockList(); err != nil {
		return err
	}

	ss.dynamicTrack.Init()

	return nil
}

func (ss *ServerState) initDRSZones() error {
	var drsZonesPath string

	if ss.raceConfig.TrackLayout == "" {
		drsZonesPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, "data", "drs_zones.ini")
	} else {
		drsZonesPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, ss.raceConfig.TrackLayout, "data", "drs_zones.ini")
	}

	ss.logger.Debugf("Loading track DRS zones from %s", drsZonesPath)

	var err error

	ss.drsZones, err = LoadDRSZones(drsZonesPath)

	if err != nil {
		return err
	}

	return nil
}

func (ss *ServerState) initFixedSetups() error {
	ss.logger.Debug("Loading fixed setups")

	ss.setups = make(map[string]Setup)

entrants:
	for _, entrant := range ss.entryList {
		if entrant.FixedSetup != "" {
			// This makes an assumption that a setup can only be applied to one car model
			if _, ok := ss.setups[entrant.FixedSetup]; ok {
				// this setup has already been loaded
				continue
			}

			setupFile, err := ini.Load(filepath.Join(ss.baseDirectory, "setups", entrant.FixedSetup))

			if err != nil {
				return err
			}

			values := make(map[string]float32)

			for _, section := range setupFile.Sections() {
				if section.Name() == "DEFAULT" {
					continue
				}

				if section.Name() == "CAR" {
					carModel := section.Key("MODEL").Value()

					if entrant.Model != carModel {
						ss.logger.Debugf("entrant (%s) car model (%s) does not match setup car model (%s), setup has not been applied", entrant.Driver.Name, entrant.Model, carModel)
						continue entrants
					}
				}

				val, err := section.Key("VALUE").Float64()

				if err != nil {
					// some sections don't have VALUE key (e.g. [__EXT_PATCH] VERSION=0.1.25-preview63), we just ignore
					// them apart from the CAR section
					continue
				}

				values[section.Name()] = float32(val)
			}

			ss.logger.Debugf("Setup %s loaded successfully", entrant.FixedSetup)

			ss.setups[entrant.FixedSetup] = Setup{
				carName: entrant.Model,
				values:  values,
				isFixed: 1, // this is ignored by the client, but hey ho it's here
			}
		}
	}

	return nil
}

func (ss *ServerState) initMOTD() error {
	ss.logger.Debugf("Loading server MOTD from: %s", ss.serverConfig.WelcomeMessageFile)

	motd, err := ioutil.ReadFile(filepath.Join(ss.baseDirectory, ss.serverConfig.WelcomeMessageFile))

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if !os.IsNotExist(err) {
		// save motd
		ss.messageOfTheDay = string(motd)

		ss.logger.Infof("Server MOTD initialised to:")
		ss.logger.Println(ss.messageOfTheDay)
	} else {
		ss.logger.Debug("Server MOTD file not found, skipping")
	}

	return nil
}

const BlockListFileName = "blocklist.json"

func (ss *ServerState) initBlockList() error {
	ss.logger.Debugf("Loading server blocklist from: %s", BlockListFileName)

	blockListFile, err := ioutil.ReadFile(filepath.Join(ss.baseDirectory, BlockListFileName))

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if !os.IsNotExist(err) {
		var blockList []string

		err := json.Unmarshal(blockListFile, &blockList)

		if err != nil {
			ss.logger.WithError(err).Errorf("Server %s is formatted incorrectly. Skipping", BlockListFileName)
		} else {
			ss.logger.Infof("Block list loaded successfully: %s", strings.Join(blockList, ", "))
			ss.blockList = blockList
		}
	} else {
		ss.logger.Debugf("Server %s not found, skipping", BlockListFileName)
	}

	return nil
}

func (ss *ServerState) CurrentTimeMillisecond() int64 {
	return time.Since(ss.serverStartTime).Milliseconds()
}

func (ss *ServerState) GetCarByName(name string) *Car {
	for _, entrant := range ss.entryList {
		if entrant.Driver.Name == name {
			return entrant
		}
	}

	return nil
}

func (ss *ServerState) GetCarByGUID(guid string, connected bool) *Car {
	for _, entrant := range ss.entryList {
		if entrant.HasGUID(guid) {
			if (connected && entrant.IsConnected()) || !connected {
				return entrant
			}
		}
	}

	return nil
}

var ErrCarNotFound = errors.New("openAcServer: car not found")

func (ss *ServerState) GetCarByID(carID CarID) (*Car, error) {
	if carID == ServerCarID {
		// @TODO better way of doing this?
		return &Car{}, nil
	}

	for _, entrant := range ss.entryList {
		if entrant.CarID == carID {
			return entrant, nil
		}
	}

	return nil, ErrCarNotFound
}

func (ss *ServerState) GetCarByTCPConn(conn net.Conn) (*Car, error) {
	for _, entrant := range ss.entryList {
		if entrant.Connection.tcpConn == conn {
			return entrant, nil
		}
	}

	return nil, ErrCarNotFound
}

func (ss *ServerState) AssociateUDPConnectionByCarID(addr net.Addr, carID CarID) error {
	car, err := ss.GetCarByID(carID)

	if err != nil {
		return err
	}

	if car.Connection.udpAddr != nil && car.Connection.udpAddr.String() == addr.String() {
		return nil
	}

	ss.logger.Infof("Associating address: %s to CarID: %d", addr.String(), carID)

	car.AssociateUDPAddress(addr)

	bw := NewPacket(nil)
	bw.Write(TCPCarConnected)
	bw.Write(carID)
	bw.WriteString(car.Driver.Name)
	bw.WriteString(car.Driver.Nation)

	ss.BroadcastOthersTCP(bw, carID)

	return nil
}

func (ss *ServerState) BroadcastAllTCP(p *Packet) {
	for _, entrant := range ss.entryList {
		if !entrant.IsConnected() {
			continue
		}

		if err := p.WriteTCP(entrant.Connection.tcpConn); err != nil {
			ss.logger.WithError(err).Errorf("Could not broadcast message to CarID: %d", entrant.CarID)
			continue
		}
	}
}

func (ss *ServerState) BroadcastOthersTCP(p *Packet, ignoreCarID CarID) {
	for _, entrant := range ss.entryList {
		if !entrant.IsConnected() || entrant.CarID == ignoreCarID {
			continue
		}

		if err := p.WriteTCP(entrant.Connection.tcpConn); err != nil {
			ss.logger.WithError(err).Errorf("Could not broadcast message to CarID: %d", entrant.CarID)
			continue
		}
	}
}

func (ss *ServerState) GetCarByUDPAddress(addr net.Addr) *Car {
	for _, entrant := range ss.entryList {
		if entrant.IsConnected() && entrant.Connection.udpAddr != nil && entrant.Connection.udpAddr.String() == addr.String() {
			return entrant
		}
	}

	return nil
}

func (ss *ServerState) Kick(carID CarID, reason KickReason) error {
	entrant, err := ss.GetCarByID(carID)

	if err != nil {
		return err
	}

	switch ss.serverConfig.BlockListMode {
	case BlockListModeNormalKick:
	case BlockListModeNoRejoin:
		ss.noJoinList[entrant.Driver.GUID] = true
	case BlockListModeAddToList:
		err := ss.AddToBlockList(entrant.Driver.GUID)

		if err != nil {
			ss.logger.WithError(err).Errorf("Kick: Couldn't add %s to the server blocklist.json", entrant.Driver.GUID)
		}
	}

	message := TCPKickMessage{
		CarID:      carID,
		KickReason: reason,
	}

	p := NewPacket(nil)
	p.Write(TCPMessageKick)
	p.Write(message)

	ss.BroadcastAllTCP(p)

	return nil
}

func (ss *ServerState) AddToBlockList(guid string) error {
	ss.logger.Debugf("Adding %s to the server blocklist.json", guid)

	ss.blockList = append(ss.blockList, guid)

	// save to file
	file, err := os.Create(filepath.Join("blocklist.json"))

	if err != nil {
		return err
	}

	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "\t")

	return encoder.Encode(ss.blockList)
}

type Chat struct {
	FromCar CarID
	ToCar   CarID
	Message string
	Time    time.Time
}

func (ss *ServerState) BroadcastChat(carID CarID, message string, rateLimit bool) {
	p := NewPacket(nil)

	p.Write(TCPMessageBroadcastChat)
	p.Write(carID)
	p.WriteUTF32String(message)

	if carID != ServerCarID {
		go func() {
			err := ss.plugin.OnChat(Chat{
				FromCar: carID,
				ToCar:   ServerCarID,
				Message: message,
				Time:    time.Now(),
			})

			if err != nil {
				ss.logger.WithError(err).Error("On chat plugin returned an error")
			}
		}()
	}

	if rateLimit {
		<-ss.broadcastChatLimiter.C
	}

	ss.BroadcastAllTCP(p)
}

func (ss *ServerState) SendChat(fromCarID CarID, toCarID CarID, message string, rateLimit bool) error {
	p := NewPacket(nil)

	p.Write(TCPMessageSendChat)
	p.Write(fromCarID)
	p.WriteUTF32String(message)

	car, err := ss.GetCarByID(toCarID)

	if err != nil {
		return err
	}

	if !car.IsConnected() {
		return nil
	}

	if rateLimit {
		<-car.Connection.chatLimiter.C
	}

	if fromCarID != ServerCarID {
		go func() {
			err := ss.plugin.OnChat(Chat{
				FromCar: fromCarID,
				ToCar:   toCarID,
				Message: message,
				Time:    time.Now(),
			})

			if err != nil {
				ss.logger.WithError(err).Error("On chat plugin returned an error")
			}
		}()
	}

	return p.WriteTCP(car.Connection.tcpConn)
}

func (ss *ServerState) ChangeTyre(car *Car, tyre string) error {
	car.ChangeTyres(tyre)

	p := NewPacket(nil)

	p.Write(TCPMessageTyreChange)
	p.Write(car.CarID)
	p.WriteString(tyre)

	ss.logger.Debugf("Car: %s changed tyres to: %s", car, tyre)

	ss.BroadcastOthersTCP(p, car.CarID)

	go func() {
		err := ss.plugin.OnTyreChange(car.Copy(), tyre)

		if err != nil {
			ss.logger.WithError(err).Error("On tyre change plugin returned an error")
		}
	}()

	return nil
}

func (ss *ServerState) CreateBoPPacket(cars []*Car) *Packet {
	bw := NewPacket(nil)
	bw.Write(TCPSendBoP)
	bw.Write(uint8(len(cars)))

	for _, car := range cars {
		bw.Write(car.CarID)
		bw.Write(car.Ballast)
		bw.Write(car.Restrictor)
	}

	return bw
}

func (ss *ServerState) BroadcastUpdateBoP(car *Car) {
	ss.logger.Infof("Broadcasting updated BoP for %s (ballast: %.0f, restrictor: %.0f) to all clients", car.String(), car.Ballast, car.Restrictor)

	var cars []*Car

	cars = append(cars, car)

	bw := ss.CreateBoPPacket(cars)

	ss.BroadcastAllTCP(bw)
}

func (ss *ServerState) SendBoP(car *Car) error {
	ss.logger.Infof("Sending BoP info to entrant: %s", car.String())

	bw := ss.CreateBoPPacket(ss.entryList)

	return bw.WriteTCP(car.Connection.tcpConn)
}

func (ss *ServerState) SendMOTD(car *Car) error {
	if ss.messageOfTheDay == "" {
		return nil
	}

	ss.logger.Infof("Sending MOTD to entrant: %s", car.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendTextFile)
	bw.Write(TCPSpacer)

	bw.WriteBigUTF32String(ss.messageOfTheDay)

	return bw.WriteTCP(car.Connection.tcpConn)
}

func (ss *ServerState) SendDRSZones(car *Car) error {
	if ss.drsZones == nil {
		return nil
	}

	ss.logger.Infof("Sending DRS Zones to entrant: %s", car.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendDRSZone)
	bw.Write(uint8(len(ss.drsZones)))

	for _, zone := range ss.drsZones {
		bw.Write(zone.Start)
		bw.Write(zone.End)
	}

	return bw.WriteTCP(car.Connection.tcpConn)
}

func (ss *ServerState) SendSetup(car *Car) error {
	if car.FixedSetup == "" {
		return nil
	}

	if _, ok := ss.setups[car.FixedSetup]; !ok {
		ss.logger.Infof("Fixed setup %s was selected for %s, but was not found on event start! Setup not applied!", car.FixedSetup, car.Driver.Name)
		return nil
	}

	ss.logger.Infof("Sending fixed setup %s to %s", car.FixedSetup, car.Driver.Name)

	bw := NewPacket(nil)
	bw.Write(TCPSendSetup)
	bw.Write(ss.setups[car.FixedSetup].isFixed)
	bw.Write(uint8(len(ss.setups[car.FixedSetup].values)))

	for key, val := range ss.setups[car.FixedSetup].values {
		bw.WriteString(key)
		bw.Write(val)
	}

	return bw.WriteTCP(car.Connection.tcpConn)
}

func (ss *ServerState) SendStatus(car *Car, currentTime int64) error {
	if !car.IsConnected() || !car.HasSentFirstUpdate() {
		return nil
	}

	var connectedCarList []*Car
	carIndex := 0

	for _, car := range ss.entryList {
		if !car.IsConnected() || !car.HasSentFirstUpdate() {
			continue
		}

		connectedCarList = append(connectedCarList, car)

		carIndex++
	}

	if err := ss.SendMegaPacket(car, currentTime, connectedCarList); err != nil {
		return err
	}

	return nil
}

func (ss *ServerState) SendMegaPacket(car *Car, currentTime int64, connectedCars []*Car) error {
	bw := NewPacket(nil)
	bw.Write(UDPMessageMegaPacket)
	bw.Write(uint32(currentTime))
	bw.Write(uint16(car.Connection.Ping))
	bw.Write(uint8(len(connectedCars) - 1))

	for _, otherCar := range connectedCars {
		if otherCar == car {
			continue
		}

		status := otherCar.Status

		if car.IsSpectator() && car.GetSpectatingCarID() == otherCar.CarID {
			// spectator cars see the true status of the car they have selected
			status = otherCar.PluginStatus
		}

		bw.Write(otherCar.CarID)
		bw.Write(status.Sequence)
		bw.Write(status.Timestamp - car.Connection.TimeOffset)
		bw.Write(uint16(otherCar.Connection.Ping))
		bw.Write(status.Position)
		bw.Write(status.Rotation)
		bw.Write(status.Velocity)
		bw.Write(status.TyreAngularSpeed)
		bw.Write(status.SteerAngle)
		bw.Write(status.WheelAngle)
		bw.Write(status.EngineRPM)
		bw.Write(status.GearIndex)
		bw.Write(status.StatusBytes)
	}

	return bw.WriteUDP(ss.udp, car.Connection.udpAddr)
}

func (ss *ServerState) BroadcastCarUpdate(car *Car) {
	for _, otherCar := range ss.entryList {
		if otherCar == car || !otherCar.IsConnected() || !otherCar.HasSentFirstUpdate() {
			continue
		}

		if !car.ShouldSendUpdate(otherCar) {
			continue
		}

		status := car.Status

		if otherCar.IsSpectator() && otherCar.GetSpectatingCarID() == car.CarID {
			// spectator cars see the true status of the car they have selected
			status = car.PluginStatus
		}

		p := NewPacket(nil)
		p.Write(UDPMessageCarUpdate)
		p.Write(car.CarID)
		p.Write(status.Sequence)
		p.Write(status.Timestamp - otherCar.Connection.TimeOffset)
		p.Write(uint16(car.Connection.Ping))
		p.Write(status.Position)
		p.Write(status.Rotation)
		p.Write(status.Velocity)
		p.Write(status.TyreAngularSpeed)
		p.Write(status.SteerAngle)
		p.Write(status.WheelAngle)
		p.Write(status.EngineRPM)
		p.Write(status.GearIndex)
		p.Write(status.StatusBytes)
		p.Write(status.PerformanceDelta)
		p.Write(status.Gas)

		if err := p.WriteUDP(ss.udp, otherCar.Connection.udpAddr); err != nil {
			ss.logger.WithError(err).Errorf("Could not send CarUpdate to %s", otherCar.String())
		}
	}
}

func (ss *ServerState) DisconnectCar(car *Car) error {
	if car == nil {
		return nil
	}

	ss.closeTCPConnection(car.Connection.tcpConn)

	ss.logger.Infof("Car: %s disconnected cleanly from the server", car)

	p := NewPacket(nil)
	p.Write(TCPBroadcastClientDisconnected)
	p.Write(car.CarID)

	ss.BroadcastAllTCP(p)

	return nil
}

func (ss *ServerState) closeTCPConnectionWithError(conn net.Conn, errorMessage MessageType) error {
	p := NewPacket(nil)
	p.Write(errorMessage)

	if err := p.WriteTCP(conn); err != nil {
		return err
	}

	ss.closeTCPConnection(conn)

	return nil
}

func (ss *ServerState) closeTCPConnection(conn net.Conn) {
	car, _ := ss.GetCarByTCPConn(conn)

	if c, ok := conn.(*tcpConn); ok {
		c.closer <- struct{}{}
	}

	if car != nil {
		car.CloseConnection()

		go func() {
			if err := ss.plugin.OnConnectionClosed(car.Copy()); err != nil {
				ss.logger.WithError(err).Error("On connection closed plugin returned an error")
			}
		}()
	} else {
		ss.logger.Debugf("Closed TCP connection: %s without finding an associated car", conn.RemoteAddr().String())
	}
}

type LeaderboardLine struct {
	Car         *Car
	Time        time.Duration
	NumLaps     int
	GapToLeader time.Duration
}

func (l *LeaderboardLine) String() string {
	return fmt.Sprintf("CarID: %d, Time: %s, NumLaps: %d", l.Car.CarID, l.Time, l.NumLaps)
}

func (ss *ServerState) Leaderboard(sessionType SessionType) []*LeaderboardLine {
	var leaderboard []*LeaderboardLine

	for _, car := range ss.entryList {
		var duration time.Duration

		lapCount := 0
		laps := car.GetLaps()

		switch sessionType {
		case SessionTypeRace:
			for _, lap := range laps {
				duration += lap.LapTime
			}

			lapCount = len(laps)
		default:
			bestLap := car.BestLap()
			duration = bestLap.LapTime

			for _, lap := range laps {
				if lap.Cuts == 0 {
					lapCount++
				}
			}
		}

		leaderboard = append(leaderboard, &LeaderboardLine{
			Car:     car,
			Time:    duration,
			NumLaps: lapCount,
		})
	}

	switch sessionType {
	case SessionTypeRace:
		sort.SliceStable(leaderboard, func(i, j int) bool {
			carI, carJ := leaderboard[i], leaderboard[j]

			if carI.NumLaps == carJ.NumLaps {
				return carI.Time < carJ.Time
			}

			return carI.NumLaps > carJ.NumLaps
		})

	default:
		sort.SliceStable(leaderboard, func(i, j int) bool {
			carI, carJ := leaderboard[i], leaderboard[j]

			if carI.Car.Driver.GUID == "" {
				// carI has had no drivers, so they are not less than J
				return false
			}

			if carJ.Car.Driver.GUID == "" {
				// carJ has had no drivers, so I is less than it
				return true
			}

			if carI.Time == carJ.Time {
				carILaps := carI.Car.GetLaps()
				carJLaps := carJ.Car.GetLaps()

				if len(carILaps) == len(carJLaps) {
					if carI.Car.IsConnected() && !carJ.Car.IsConnected() {
						return true
					} else if !carI.Car.IsConnected() && carJ.Car.IsConnected() {
						return false
					}

					return carI.Car.CarID < carJ.Car.CarID
				}

				return len(carILaps) > len(carJLaps)
			}

			return carI.Time < carJ.Time
		})
	}

	if len(leaderboard) > 0 {
		leader := leaderboard[0]

		for _, line := range leaderboard {
			line.GapToLeader = line.Time - leader.Time
		}
	}

	ss.plugin.SortLeaderboard(sessionType, leaderboard)

	return leaderboard
}

func (ss *ServerState) Close() {
	ss.broadcastChatLimiter.Stop()
}
