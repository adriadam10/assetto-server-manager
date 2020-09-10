package acserver

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"regexp"
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
	entryList       EntryList
	raceConfig      *EventConfig
	serverConfig    *ServerConfig
	customChecksums []CustomChecksumFile
	plugin          Plugin
	logger          Logger

	packetConn    net.PacketConn
	baseDirectory string

	// modifiable
	sunAngle   float32
	randomSeed uint32

	currentWeather      *CurrentWeather
	currentSessionIndex uint8
	currentSession      SessionConfig

	// fixed
	drsZones           map[string]DRSZone
	setups             map[string]Setup
	checkSummableFiles []checksumFile
	messageOfTheDay    string
	blockList          []string
	noJoinList         map[string]bool
}

type Setup struct {
	carName string
	isFixed uint8 // this is ignored by the client, but hey ho it's here
	values  map[string]float32
}

type DRSZone struct {
	detection float32
	start     float32
	end       float32
}

func NewServerState(baseDirectory string, serverConfig *ServerConfig, raceConfig *EventConfig, entryList EntryList, checksums []CustomChecksumFile, plugin Plugin, logger Logger) (*ServerState, error) {
	ss := &ServerState{
		serverConfig:    serverConfig,
		raceConfig:      raceConfig,
		entryList:       entryList,
		customChecksums: checksums,
		plugin:          plugin,
		logger:          logger,
		sunAngle:        raceConfig.SunAngle,
		randomSeed:      rand.Uint32(),
		noJoinList:      make(map[string]bool),
		baseDirectory:   baseDirectory,
	}

	if err := ss.init(); err != nil {
		return nil, err
	}

	return ss, nil
}

func (ss *ServerState) init() error {
	if err := ss.initChecksums(); err != nil {
		return err
	}

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

	ss.raceConfig.DynamicTrack.Init(ss.logger)

	return nil
}

var systemDataSurfacesPath = filepath.Join("system", "data", "surfaces.ini")

type checksumFile struct {
	Filename string
	MD5      []byte
}

type CustomChecksumFile struct {
	Name     string `json:"name" yaml:"name"`
	Filename string `json:"file_path" yaml:"file_path"`
	MD5      string `json:"md5" yaml:"md5"`
}

func (ss *ServerState) initChecksums() error {
	var trackSurfacesPath string
	var trackModelsPath string

	if ss.raceConfig.TrackLayout == "" {
		trackSurfacesPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, "data", "surfaces.ini")
		trackModelsPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, "models.ini")
	} else {
		trackSurfacesPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, ss.raceConfig.TrackLayout, "data", "surfaces.ini")
		trackModelsPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, "models_"+ss.raceConfig.TrackLayout+".ini")
	}

	filesToChecksum := []string{
		filepath.Join(ss.baseDirectory, systemDataSurfacesPath),
		trackSurfacesPath,
		trackModelsPath,
	}

	for _, car := range ss.raceConfig.Cars {
		acdFilepath := filepath.Join(ss.baseDirectory, "content", "cars", car, "data.acd")

		if _, err := os.Stat(acdFilepath); os.IsNotExist(err) {
			// this car is likely using a data folder rather than an acd file. checksum all files within the data path
			dataPath := filepath.Join(ss.baseDirectory, "content", "cars", car, "data")

			files, err := ioutil.ReadDir(dataPath)

			if err != nil {
				return err
			}

			for _, file := range files {
				filesToChecksum = append(filesToChecksum, dataPath+file.Name())
			}
		} else if err != nil {
			return err
		} else {
			filesToChecksum = append(filesToChecksum, acdFilepath)
		}
	}

	ss.logger.Debugf("Running checksum for %d files", len(filesToChecksum)+len(ss.customChecksums))

	for _, file := range filesToChecksum {
		checksum, err := md5File(file)

		if os.IsNotExist(err) {
			ss.logger.Warnf("Could not find checksum file: %s", file)
			continue
		} else if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(ss.baseDirectory, file)

		if err != nil {
			return err
		}

		relativePath = filepath.ToSlash(relativePath)

		ss.logger.Debugf("Checksum added: md5(%s)=%s", relativePath, hex.EncodeToString(checksum))

		ss.checkSummableFiles = append(ss.checkSummableFiles, checksumFile{Filename: relativePath, MD5: checksum})
	}

	for _, customChecksum := range ss.customChecksums {
		if !sanitiseChecksumPath(customChecksum.Filename) {
			continue
		}

		checksum, err := hex.DecodeString(customChecksum.MD5)

		if err != nil {
			ss.logger.WithError(err).Errorf("Couldn't decode checksum: %s", customChecksum.MD5)
		} else {
			ss.logger.Debugf("Checksum added from config: md5(%s)=%s", customChecksum.Filename, customChecksum.MD5)

			ss.checkSummableFiles = append(ss.checkSummableFiles, checksumFile{customChecksum.Filename, checksum})
		}
	}

	return nil
}

var absPathRegex = regexp.MustCompile(`[A-Z]:`)

func sanitiseChecksumPath(path string) bool {
	cleanPath := filepath.Clean(path)

	if strings.HasPrefix(cleanPath, "..") {
		return false
	}

	if strings.HasPrefix(cleanPath, "\\") {
		return false
	}

	if filepath.IsAbs(cleanPath) {
		return false
	}

	if absPathRegex.MatchString(cleanPath) {
		return false
	}

	return true
}

func md5File(filepath string) ([]byte, error) {
	f, err := os.Open(filepath)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	h := md5.New()

	_, err = io.Copy(h, f)

	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func (ss *ServerState) initDRSZones() error {
	var drsZonesPath string

	if ss.raceConfig.TrackLayout == "" {
		drsZonesPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, "data", "drs_zones.ini")
	} else {
		drsZonesPath = filepath.Join(ss.baseDirectory, "content", "tracks", ss.raceConfig.Track, ss.raceConfig.TrackLayout, "data", "drs_zones.ini")
	}

	ss.logger.Debugf("Loading track DRS zones from %s", drsZonesPath)

	drsFile, err := ini.Load(drsZonesPath)

	if err != nil {
		return err
	}

	ss.drsZones = make(map[string]DRSZone)

	for _, section := range drsFile.Sections() {
		if section.Name() == "DEFAULT" {
			continue
		}

		detection, err := section.Key("DETECTION").Float64()

		if err != nil {
			ss.logger.WithError(err).Errorf("Could not load DETECTION for %s of %s", section.Name(), drsZonesPath)
			continue
		}

		start, err := section.Key("START").Float64()

		if err != nil {
			ss.logger.WithError(err).Errorf("Could not load START for %s of %s", section.Name(), drsZonesPath)
			continue
		}

		end, err := section.Key("END").Float64()

		if err != nil {
			ss.logger.WithError(err).Errorf("Could not load END for %s of %s", section.Name(), drsZonesPath)
			continue
		}

		ss.drsZones[section.Name()] = DRSZone{
			detection: float32(detection),
			start:     float32(start),
			end:       float32(end),
		}
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

			setupFile, err := ini.Load(filepath.Join(ss.baseDirectory, entrant.FixedSetup))

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

func (ss *ServerState) initBlockList() error {
	ss.logger.Debug("Loading server blocklist.json")

	blockListFile, err := ioutil.ReadFile(filepath.Join(ss.baseDirectory, "blocklist.json"))

	if err != nil && !os.IsNotExist(err) {
		return err
	}

	if !os.IsNotExist(err) {
		var blockList []string

		err := json.Unmarshal(blockListFile, &blockList)

		if err != nil {
			ss.logger.Debug("Server blocklist.json is formatted incorrectly! Skipping")
		} else {
			ss.logger.Debugf("Block list loaded successfully: %s", strings.Join(blockList, ", "))
			ss.blockList = blockList
		}
	} else {
		ss.logger.Debug("Server blocklist.json not found, skipping")
	}

	return nil
}

func (ss *ServerState) ChangeWeather(weatherConfig *WeatherConfig) {
	ss.currentWeather = &CurrentWeather{
		Ambient:       uint8(weatherConfig.BaseTemperatureAmbient),
		Road:          uint8(weatherConfig.BaseTemperatureAmbient + weatherConfig.BaseTemperatureRoad),
		GraphicsName:  weatherConfig.Graphics,
		WindSpeed:     int16(weatherConfig.WindSpeed),
		WindDirection: int16(weatherConfig.WindDirection),
	}

	for _, car := range ss.entryList {
		if !car.IsConnected() {
			continue
		}

		if err := ss.SendWeather(car); err != nil {
			ss.logger.WithError(err).Errorf("Could not send weather to car: %s", car.String())
		}
	}

	go func() {
		err := ss.plugin.OnWeatherChange(*ss.currentWeather)

		if err != nil {
			ss.logger.WithError(err).Error("On weather change plugin returned an error")
		}
	}()
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
	ss.logger.Infof("Associating address: %s to CarID: %d", addr.String(), carID)
	entrant, err := ss.GetCarByID(carID)

	if err != nil {
		return err
	}

	entrant.Connection.udpAddr = addr

	bw := NewPacket(nil)
	bw.Write(TCPCarConnected)
	bw.Write(carID)
	bw.WriteString(entrant.Driver.Name)
	bw.WriteString(entrant.Driver.Nation)

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

func (ss *ServerState) BroadcastChat(carID CarID, message string) {
	p := NewPacket(nil)

	p.Write(TCPMessageBroadcastChat)
	p.Write(carID)
	p.WriteUTF32String(message)

	/* @TODO do we want to call on chat for broadcasted messages?
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
	}()*/

	ss.BroadcastAllTCP(p)
}

func (ss *ServerState) SendChat(fromCarID CarID, toCarID CarID, message string) error {
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

func (ss *ServerState) BroadcastDamageZones(entrant *Car) error {
	if ss.currentSession.SessionType == SessionTypeQualifying && ss.currentSession.Solo {
		return nil
	}

	p := NewPacket(nil)

	p.Write(TCPMessageDamageZones)
	p.Write(entrant.CarID)
	p.Write(entrant.DamageZones)

	ss.BroadcastOthersTCP(p, entrant.CarID)

	return nil
}

func (ss *ServerState) ChangeTyre(carID CarID, tyre string) error {
	p := NewPacket(nil)

	p.Write(TCPMessageTyreChange)
	p.Write(carID)
	p.WriteString(tyre)

	entrant, err := ss.GetCarByID(carID)

	if err != nil {
		return err
	}

	entrant.Tyres = tyre

	ss.logger.Debugf("Car: %s changed tyres to: %s", entrant, tyre)

	ss.BroadcastOthersTCP(p, entrant.CarID)

	return nil
}

func (ss *ServerState) CompleteLap(carID CarID, lap *LapCompleted, target *Car) error {
	if carID != ServerCarID {
		ss.logger.Infof("CarID: %d just completed lap: %s (%d cuts) (splits: %v)", carID, time.Duration(lap.LapTime)*time.Millisecond, lap.Cuts, lap.Splits)
		ss.currentSession.numCompletedLaps++
		ss.raceConfig.DynamicTrack.OnLapCompleted()
	}

	entrant, err := ss.GetCarByID(carID)

	if err != nil {
		return err
	}

	if entrant.SessionData.HasCompletedSession {
		// entrants which have completed the session can't complete more laps
		return nil
	}

	l := entrant.AddLap(lap)

	if carID != ServerCarID {
		go func() {
			err := ss.plugin.OnLapCompleted(entrant.CarID, *l)

			if err != nil {
				ss.logger.WithError(err).Error("On lap completed plugin returned an error")
			}
		}()
	}

	leaderboard := ss.Leaderboard()

	if ss.currentSession.Laps > 0 {
		entrant.SessionData.HasCompletedSession = entrant.SessionData.LapCount == int(ss.currentSession.Laps)
	} else {
		if currentTimeMillisecond() > ss.currentSession.FinishTime() {
			leader := leaderboard[0]

			if ss.raceConfig.RaceExtraLap {
				if entrant.SessionData.HasExtraLapToGo {
					// everyone at this point has completed their extra lap
					entrant.SessionData.HasCompletedSession = true
				} else {
					// the entrant has another lap to go if they are the leader, or the leader has an extra lap to go
					entrant.SessionData.HasExtraLapToGo = leader.Car == entrant || leader.Car.SessionData.HasExtraLapToGo
				}
			} else {
				// the entrant has completed the session if they are the leader or the leader has completed the session.
				entrant.SessionData.HasCompletedSession = leader.Car == entrant || leader.Car.SessionData.HasCompletedSession
			}
		}
	}

	bw := NewPacket(nil)
	bw.Write(TCPMessageLapCompleted)
	bw.Write(carID)
	bw.Write(lap.LapTime)
	bw.Write(lap.Cuts)
	bw.Write(uint8(len(ss.entryList)))

	for _, leaderBoardLine := range leaderboard {
		bw.Write(leaderBoardLine.Car.CarID)

		switch ss.currentSession.SessionType {
		case SessionTypeRace:
			bw.Write(uint32(leaderBoardLine.Time.Milliseconds()))
		default:
			bw.Write(uint32(leaderBoardLine.Time.Milliseconds()))
		}

		bw.Write(uint16(leaderBoardLine.Car.SessionData.LapCount))
		bw.Write(leaderBoardLine.Car.SessionData.HasCompletedSession)
	}

	bw.Write(ss.raceConfig.DynamicTrack.CurrentGrip)

	if target != nil {
		return bw.WriteTCP(target.Connection.tcpConn)
	}

	ss.BroadcastAllTCP(bw)

	return nil
}

func (ss *ServerState) SendWeather(entrant *Car) error {
	ss.logger.Infof("Sending Weather (%s), to entrant: %s", ss.currentWeather.String(), entrant.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendWeather)
	bw.Write(ss.currentWeather.Ambient)
	bw.Write(ss.currentWeather.Road)
	bw.WriteUTF32String(ss.currentWeather.GraphicsName)
	bw.Write(ss.currentWeather.WindSpeed)
	bw.Write(ss.currentWeather.WindDirection)

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (ss *ServerState) SendSunAngle() {
	ss.logger.Debugf("Broadcasting Sun Angle (%.2f)", ss.sunAngle)

	bw := NewPacket(nil)
	bw.Write(TCPSendSunAngle)
	bw.Write(ss.sunAngle)

	ss.BroadcastAllTCP(bw)
}

func (ss *ServerState) CreateBoPPacket(entrants []*Car) *Packet {
	bw := NewPacket(nil)
	bw.Write(TCPSendBoP)
	bw.Write(uint8(len(entrants)))

	for _, entrant := range entrants {
		bw.Write(entrant.CarID)
		bw.Write(entrant.Ballast)
		bw.Write(entrant.Restrictor)
	}

	return bw
}

func (ss *ServerState) BroadcastUpdateBoP(entrant *Car) {
	ss.logger.Infof("Broadcasting updated BoP for %s (ballast: %.0f, restrictor: %.0f) to all clients", entrant.String(), entrant.Ballast, entrant.Restrictor)

	var entrants []*Car

	entrants = append(entrants, entrant)

	bw := ss.CreateBoPPacket(entrants)

	ss.BroadcastAllTCP(bw)
}

func (ss *ServerState) SendBoP(entrant *Car) error {
	ss.logger.Infof("Sending BoP info to entrant: %s", entrant.String())

	bw := ss.CreateBoPPacket(ss.entryList)

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (ss *ServerState) SendMOTD(entrant *Car) error {
	if ss.messageOfTheDay == "" {
		return nil
	}

	ss.logger.Infof("Sending MOTD to entrant: %s", entrant.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendTextFile)
	bw.Write(TCPSpacer)

	bw.WriteBigUTF32String(ss.messageOfTheDay)

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (ss *ServerState) SendDRSZones(entrant *Car) error {
	if ss.drsZones == nil {
		return nil
	}

	ss.logger.Infof("Sending DRS Zones to entrant: %s", entrant.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendDRSZone)
	bw.Write(uint8(len(ss.drsZones)))

	for _, zone := range ss.drsZones {
		bw.Write(zone.start)
		bw.Write(zone.end)
	}

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (ss *ServerState) SendSetup(entrant *Car) error {
	if entrant.FixedSetup == "" {
		return nil
	}

	if _, ok := ss.setups[entrant.FixedSetup]; !ok {
		ss.logger.Infof("Fixed setup %s was selected for %s, but was not found on event start! Setup not applied!", entrant.FixedSetup, entrant.Driver.Name)
		return nil
	}

	ss.logger.Infof("Sending fixed setup %s to %s", entrant.FixedSetup, entrant.Driver.Name)

	bw := NewPacket(nil)
	bw.Write(TCPSendSetup)
	bw.Write(ss.setups[entrant.FixedSetup].isFixed)
	bw.Write(uint8(len(ss.setups[entrant.FixedSetup].values)))

	for key, val := range ss.setups[entrant.FixedSetup].values {
		bw.WriteString(key)
		bw.Write(val)
	}

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (ss *ServerState) SendStatus(car *Car, currentTime int64) error {
	if !car.IsConnected() || !car.Connection.HasSentFirstUpdate {
		return nil
	}

	var connectedCarList []*Car
	carIndex := 0

	for _, car := range ss.entryList {
		if !car.IsConnected() || !car.Connection.HasSentFirstUpdate {
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

		bw.Write(otherCar.CarID)
		bw.Write(otherCar.Status.Sequence)
		bw.Write(otherCar.Status.Timestamp - car.Connection.TimeOffset)
		bw.Write(uint16(otherCar.Connection.Ping))
		bw.Write(otherCar.Status.Position)
		bw.Write(otherCar.Status.Rotation)
		bw.Write(otherCar.Status.Velocity)
		bw.Write(otherCar.Status.TyreAngularSpeed)
		bw.Write(otherCar.Status.SteerAngle)
		bw.Write(otherCar.Status.WheelAngle)
		bw.Write(otherCar.Status.EngineRPM)
		bw.Write(otherCar.Status.GearIndex)
		bw.Write(otherCar.Status.StatusBytes)
	}

	return bw.WriteUDP(ss.packetConn, car.Connection.udpAddr)
}

func (ss *ServerState) BroadcastCarUpdate(car *Car) {
	for _, otherCar := range ss.entryList {
		if otherCar == car || !otherCar.IsConnected() || !otherCar.Connection.HasSentFirstUpdate {
			continue
		}

		if !car.ShouldSendUpdate(otherCar) {
			continue
		}

		p := NewPacket(nil)
		p.Write(UDPMessageCarUpdate)
		p.Write(car.CarID)
		p.Write(car.Status.Sequence)
		p.Write(car.Status.Timestamp - otherCar.Connection.TimeOffset)
		p.Write(uint16(car.Connection.Ping))
		p.Write(car.Status.Position)
		p.Write(car.Status.Rotation)
		p.Write(car.Status.Velocity)
		p.Write(car.Status.TyreAngularSpeed)
		p.Write(car.Status.SteerAngle)
		p.Write(car.Status.WheelAngle)
		p.Write(car.Status.EngineRPM)
		p.Write(car.Status.GearIndex)
		p.Write(car.Status.StatusBytes)
		p.Write(car.Status.PerformanceDelta)
		p.Write(car.Status.Gas)

		if err := p.WriteUDP(ss.packetConn, otherCar.Connection.udpAddr); err != nil {
			ss.logger.WithError(err).Errorf("Could not send CarUpdate to %s", otherCar.String())
		}
	}
}

func (ss *ServerState) DisconnectCar(car *Car) error {
	if car == nil {
		return nil
	}

	car.Connection.Close()

	ss.logger.Infof("Car: %s disconnected cleanly from the server", car)

	p := NewPacket(nil)
	p.Write(TCPBroadcastClientDisconnected)
	p.Write(car.CarID)

	ss.BroadcastAllTCP(p)

	go func() {
		err := ss.plugin.OnConnectionClosed(*car)

		if err != nil {
			ss.logger.WithError(err).Error("On connection closed plugin returned an error")
		}
	}()

	return nil
}

func (ss *ServerState) SendSessionInfo(entrant *Car, leaderBoard []*LeaderboardLine) error {
	if leaderBoard == nil {
		leaderBoard = ss.Leaderboard()
	}

	ss.logger.Debugf("Sending Client Session Information")

	bw := NewPacket(nil)
	bw.Write(TCPMessageCurrentSessionInfo)
	bw.WriteString(ss.currentSession.Name)
	bw.Write(ss.currentSessionIndex)                 // session index
	bw.Write(ss.currentSession.SessionType)          // type
	bw.Write(ss.currentSession.Time)                 // time
	bw.Write(ss.currentSession.Laps)                 // laps
	bw.Write(ss.raceConfig.DynamicTrack.CurrentGrip) // dynamic track, grip

	for _, leaderboardLine := range leaderBoard {
		bw.Write(leaderboardLine.Car.CarID)
	}

	bw.Write(ss.currentSession.startTime - int64(entrant.Connection.TimeOffset))

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (ss *ServerState) BroadcastSessionCompleted() {
	ss.logger.Infof("Broadcasting session completed packet for session: %s", ss.currentSession.SessionType)
	p := NewPacket(nil)
	p.Write(TCPMessageSessionCompleted)

	for _, leaderboardLine := range ss.Leaderboard() {
		p.Write(leaderboardLine.Car.CarID)
		p.Write(uint32(leaderboardLine.Time.Milliseconds()))
		p.Write(uint16(leaderboardLine.NumLaps))
	}

	// this bool here was previously used by Kunos to indicate to kick all users out post-session if loop mode was on
	// i'd like us not to require this if at all possible, so hopefully we can ignore it for now and just return '1'
	// (i.e. car can stay in server as sessions cycle)
	p.Write(uint8(1))

	ss.BroadcastAllTCP(p)
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

func (ss *ServerState) Leaderboard() []*LeaderboardLine {
	var leaderboard []*LeaderboardLine

	for _, car := range ss.entryList {
		var duration time.Duration

		switch ss.currentSession.SessionType {
		case SessionTypeRace:
			for _, lap := range car.SessionData.Laps {
				duration += lap.LapTime
			}
		default:
			bestLap := car.BestLap()

			if bestLap.Cuts > 0 {
				duration = maximumLapTime
			} else {
				duration = bestLap.LapTime
			}
		}

		leaderboard = append(leaderboard, &LeaderboardLine{
			Car:     car,
			Time:    duration,
			NumLaps: car.SessionData.LapCount,
		})
	}

	switch ss.currentSession.SessionType {
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
				return carI.Car.CarID < carJ.Car.CarID
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

	return leaderboard
}

func (ss *ServerState) BroadcastSessionStart(startTime int64) {
	if ss.entryList.NumConnected() == 0 {
		return
	}

	ss.logger.Infof("Broadcasting Session Start packet")

	for _, entrant := range ss.entryList {
		if entrant.IsConnected() && entrant.Connection.HasSentFirstUpdate {
			p := NewPacket(nil)
			p.Write(TCPMessageSessionStart)
			p.Write(int32(ss.currentSession.startTime - int64(entrant.Connection.TimeOffset)))
			p.Write(uint32(startTime - int64(entrant.Connection.TimeOffset)))
			p.Write(uint16(entrant.Connection.Ping))

			if err := p.WriteTCP(entrant.Connection.tcpConn); err != nil {
				ss.logger.WithError(err).Errorf("Could not send race start packet to %s", entrant.String())
			}
		}
	}
}
