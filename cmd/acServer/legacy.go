package main

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/cj123/ini"

	"justapengu.in/acsm/internal/acserver"
)

type ServerConfig struct {
	Name                      string `ini:"NAME"`
	Password                  string `ini:"PASSWORD"`
	AdminPassword             string `ini:"ADMIN_PASSWORD"`
	UDPPort                   int    `ini:"UDP_PORT"`
	TCPPort                   int    `ini:"TCP_PORT"`
	HTTPPort                  int    `ini:"HTTP_PORT"`
	UDPPluginLocalPort        int    `ini:"UDP_PLUGIN_LOCAL_PORT"`
	UDPPluginAddress          string `ini:"UDP_PLUGIN_ADDRESS"`
	AuthPluginAddress         string `ini:"AUTH_PLUGIN_ADDRESS"`
	RegisterToLobby           int    `ini:"REGISTER_TO_LOBBY"`
	ClientSendIntervalInHertz int    `ini:"CLIENT_SEND_INTERVAL_HZ"`
	SendBufferSize            int    `ini:"SEND_BUFFER_SIZE"`
	ReceiveBufferSize         int    `ini:"RECV_BUFFER_SIZE"`
	KickQuorum                int    `ini:"KICK_QUORUM"`
	VotingQuorum              int    `ini:"VOTING_QUORUM"`
	VoteDuration              int    `ini:"VOTE_DURATION"`
	BlacklistMode             int    `ini:"BLACKLIST_MODE"`
	NumberOfThreads           int    `ini:"NUM_THREADS"`
	WelcomeMessage            string `ini:"WELCOME_MESSAGE"`

	SleepTime int `ini:"SLEEP_TIME"`

	Cars                      string `ini:"CARS"`
	Track                     string `ini:"TRACK"`
	TrackLayout               string `ini:"CONFIG_TRACK"`
	SunAngle                  int    `ini:"SUN_ANGLE"`
	LegalTyres                string `ini:"LEGAL_TYRES"`
	FuelRate                  int    `ini:"FUEL_RATE"`
	DamageMultiplier          int    `ini:"DAMAGE_MULTIPLIER"`
	TyreWearRate              int    `ini:"TYRE_WEAR_RATE"`
	AllowedTyresOut           int    `ini:"ALLOWED_TYRES_OUT"`
	ABSAllowed                int    `ini:"ABS_ALLOWED"`
	TractionControlAllowed    int    `ini:"TC_ALLOWED"`
	StabilityControlAllowed   int    `ini:"STABILITY_ALLOWED"`
	AutoClutchAllowed         int    `ini:"AUTOCLUTCH_ALLOWED"`
	TyreBlanketsAllowed       int    `ini:"TYRE_BLANKETS_ALLOWED"`
	ForceVirtualMirror        int    `ini:"FORCE_VIRTUAL_MIRROR"`
	RacePitWindowStart        int    `ini:"RACE_PIT_WINDOW_START"`
	RacePitWindowEnd          int    `ini:"RACE_PIT_WINDOW_END"`
	ReversedGridRacePositions int    `ini:"REVERSED_GRID_RACE_POSITIONS"`
	TimeOfDayMultiplier       int    `ini:"TIME_OF_DAY_MULT"`
	QualifyMaxWaitPercentage  int    `ini:"QUALIFY_MAX_WAIT_PERC"`
	RaceGasPenaltyDisabled    int    `ini:"RACE_GAS_PENALTY_DISABLED"`
	MaxBallastKilograms       int    `ini:"MAX_BALLAST_KG"`
	RaceExtraLap              int    `ini:"RACE_EXTRA_LAP"`
	MaxContactsPerKilometer   int    `ini:"MAX_CONTACTS_PER_KM"`
	ResultScreenTime          int    `ini:"RESULT_SCREEN_TIME"`

	PickupModeEnabled int `ini:"PICKUP_MODE_ENABLED"`
	LockedEntryList   int `ini:"LOCKED_ENTRY_LIST"`
	LoopMode          int `ini:"LOOP_MODE"`

	MaxClients   int `ini:"MAX_CLIENTS"`
	RaceOverTime int `ini:"RACE_OVER_TIME"`
	StartRule    int `ini:"START_RULE"`

	DynamicTrack DynamicTrackConfig `ini:"-"`

	Sessions Sessions                  `ini:"-"`
	Weather  map[string]*WeatherConfig `ini:"-"`
}

type SessionType string

const (
	SessionTypeBooking    SessionType = "BOOK"
	SessionTypePractice   SessionType = "PRACTICE"
	SessionTypeQualifying SessionType = "QUALIFY"
	SessionTypeRace       SessionType = "RACE"
)

var allSessions = []SessionType{
	SessionTypeRace,
	SessionTypeQualifying,
	SessionTypePractice,
	SessionTypeBooking,
}

type Sessions map[SessionType]*SessionConfig

type SessionConfig struct {
	Type     SessionType
	Name     string `ini:"NAME"`
	Time     int    `ini:"TIME"`
	Laps     int    `ini:"LAPS"`
	IsOpen   int    `ini:"IS_OPEN"`
	WaitTime int    `ini:"WAIT_TIME"`
}

func (s Sessions) AsSlice() []*SessionConfig {
	var out []*SessionConfig

	for i := len(allSessions) - 1; i >= 0; i-- {
		sessionType := allSessions[i]

		if x, ok := s[sessionType]; ok {
			x.Type = sessionType
			out = append(out, x)
		}
	}

	return out
}

type WeatherConfig struct {
	Graphics               string `ini:"GRAPHICS"`
	BaseTemperatureAmbient int    `ini:"BASE_TEMPERATURE_AMBIENT"`
	BaseTemperatureRoad    int    `ini:"BASE_TEMPERATURE_ROAD"`
	VariationAmbient       int    `ini:"VARIATION_AMBIENT"`
	VariationRoad          int    `ini:"VARIATION_ROAD"`

	WindBaseSpeedMin       int `ini:"WIND_BASE_SPEED_MIN"`
	WindBaseSpeedMax       int `ini:"WIND_BASE_SPEED_MAX"`
	WindBaseDirection      int `ini:"WIND_BASE_DIRECTION"`
	WindVariationDirection int `ini:"WIND_VARIATION_DIRECTION"`
}

type DynamicTrackConfig struct {
	SessionStart    int `ini:"SESSION_START"`
	Randomness      int `ini:"RANDOMNESS"`
	SessionTransfer int `ini:"SESSION_TRANSFER"`
	LapGain         int `ini:"LAP_GAIN"`
}

type Entrant struct {
	Name string `ini:"DRIVERNAME"`
	Team string `ini:"TEAM"`
	GUID string `ini:"GUID"`

	Model string `ini:"MODEL"`
	Skin  string `ini:"SKIN"`

	Ballast       int    `ini:"BALLAST"`
	SpectatorMode int    `ini:"SPECTATOR_MODE"`
	Restrictor    int    `ini:"RESTRICTOR"`
	FixedSetup    string `ini:"FIXED_SETUP"`
}

func readLegacyConfigs() (*TempConfig, error) {
	b, err := ioutil.ReadFile(filepath.Join("cfg", "server_cfg.ini"))

	if err != nil {
		return nil, err
	}

	i, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment: true,
	}, b)

	if err != nil {
		return nil, err
	}

	server, err := i.GetSection("SERVER")

	if err != nil {
		return nil, err
	}

	var sc ServerConfig
	sc.Sessions = make(Sessions)
	sc.Weather = make(map[string]*WeatherConfig)

	if err := server.MapTo(&sc); err != nil {
		return nil, err
	}

	dts, err := i.GetSection("DYNAMIC_TRACK")

	if err != nil {
		return nil, err
	}

	if err := dts.MapTo(&sc.DynamicTrack); err != nil {
		return nil, err
	}

	for _, session := range allSessions {
		if sessconfig, err := i.GetSection(string(session)); err == nil {
			sc.Sessions[session] = &SessionConfig{}

			if err := sessconfig.MapTo(sc.Sessions[session]); err != nil {
				return nil, err
			}
		}
	}

	x := 0

	for {
		key := fmt.Sprintf("WEATHER_%d", x)

		weather, err := i.GetSection(key)

		if err != nil {
			break
		}

		sc.Weather[key] = &WeatherConfig{}

		if err := weather.MapTo(sc.Weather[key]); err != nil {
			return nil, err
		}

		x++
	}

	serverConfig := &acserver.ServerConfig{
		Name:                      sc.Name,
		Password:                  sc.Password,
		AdminPassword:             sc.AdminPassword,
		UDPPort:                   uint16(sc.UDPPort),
		TCPPort:                   uint16(sc.TCPPort),
		HTTPPort:                  uint16(sc.HTTPPort),
		RegisterToLobby:           sc.RegisterToLobby == 1,
		ClientSendIntervalInHertz: uint8(sc.ClientSendIntervalInHertz),
		SendBufferSize:            sc.SendBufferSize,
		ReceiveBufferSize:         sc.ReceiveBufferSize,
		KickQuorum:                sc.KickQuorum,
		VotingQuorum:              sc.VotingQuorum,
		VoteDuration:              sc.VoteDuration,
		BlockListMode:             acserver.BlockListMode(sc.BlacklistMode),
		NumberOfThreads:           sc.NumberOfThreads,
		SleepTime:                 sc.SleepTime,
		UDPPluginAddress:          sc.UDPPluginAddress,
		UDPPluginLocalPort:        sc.UDPPluginLocalPort,
		WelcomeMessageFile:        sc.WelcomeMessage,
	}

	eventConfig := &acserver.EventConfig{
		Cars:                      strings.Split(sc.Cars, ";"),
		Track:                     sc.Track,
		TrackLayout:               sc.TrackLayout,
		SunAngle:                  float32(sc.SunAngle),
		LegalTyres:                strings.Split(sc.LegalTyres, ";"),
		FuelRate:                  float32(sc.FuelRate),
		DamageMultiplier:          float32(sc.DamageMultiplier),
		TyreWearRate:              float32(sc.TyreWearRate),
		AllowedTyresOut:           int16(sc.AllowedTyresOut),
		ABSAllowed:                acserver.Assist(sc.ABSAllowed),
		TractionControlAllowed:    acserver.Assist(sc.TractionControlAllowed),
		StabilityControlAllowed:   sc.StabilityControlAllowed == 1,
		AutoClutchAllowed:         sc.AutoClutchAllowed == 1,
		TyreBlanketsAllowed:       sc.TyreBlanketsAllowed == 1,
		ForceVirtualMirror:        sc.ForceVirtualMirror == 1,
		RacePitWindowStart:        uint16(sc.RacePitWindowStart),
		RacePitWindowEnd:          uint16(sc.RacePitWindowEnd),
		ReversedGridRacePositions: int16(sc.ReversedGridRacePositions),
		TimeOfDayMultiplier:       sc.TimeOfDayMultiplier,
		QualifyMaxWaitPercentage:  sc.QualifyMaxWaitPercentage,
		RaceGasPenaltyDisabled:    sc.RaceGasPenaltyDisabled == 1,
		MaxBallastKilograms:       sc.MaxBallastKilograms,
		RaceExtraLap:              sc.RaceExtraLap == 1,
		MaxContactsPerKilometer:   uint8(sc.MaxContactsPerKilometer),
		ResultScreenTime:          uint32(sc.ResultScreenTime),
		PickupModeEnabled:         sc.PickupModeEnabled == 1,
		LockedEntryList:           sc.LockedEntryList == 1,
		LoopMode:                  sc.LoopMode == 1,
		MaxClients:                sc.MaxClients,
		RaceOverTime:              uint32(sc.RaceOverTime),
		StartRule:                 acserver.StartRule(sc.StartRule),
		DynamicTrack: acserver.DynamicTrack{
			SessionStart:    sc.DynamicTrack.SessionStart,
			Randomness:      sc.DynamicTrack.Randomness,
			SessionTransfer: sc.DynamicTrack.SessionTransfer,
			LapGain:         sc.DynamicTrack.LapGain,
		},
	}

	for _, session := range sc.Sessions.AsSlice() {
		var st acserver.SessionType

		switch session.Type {
		case SessionTypeRace:
			st = acserver.SessionTypeRace
		case SessionTypeQualifying:
			st = acserver.SessionTypeQualifying
		case SessionTypePractice:
			st = acserver.SessionTypePractice
		case SessionTypeBooking:
			st = acserver.SessionTypeBooking
		}

		eventConfig.Sessions = append(eventConfig.Sessions, &acserver.SessionConfig{
			SessionType: st,
			Name:        session.Name,
			Time:        uint16(session.Time),
			Laps:        uint16(session.Laps),
			IsOpen:      acserver.OpenRule(session.IsOpen),
			WaitTime:    session.WaitTime,
		})
	}

	for _, weather := range sc.Weather {
		eventConfig.Weather = append(eventConfig.Weather, &acserver.WeatherConfig{
			Graphics:               weather.Graphics,
			BaseTemperatureAmbient: weather.BaseTemperatureAmbient,
			BaseTemperatureRoad:    weather.BaseTemperatureRoad,
			VariationAmbient:       weather.VariationAmbient,
			VariationRoad:          weather.VariationRoad,
			WindBaseSpeedMin:       weather.WindBaseSpeedMin,
			WindBaseSpeedMax:       weather.WindBaseSpeedMax,
			WindBaseDirection:      weather.WindBaseDirection,
			WindVariationDirection: weather.WindVariationDirection,
		})
	}

	b, err = ioutil.ReadFile(filepath.Join("cfg", "entry_list.ini"))

	if err != nil {
		return nil, err
	}

	e, err := ini.LoadSources(ini.LoadOptions{
		IgnoreInlineComment: true,
	}, b)

	if err != nil {
		return nil, err
	}

	var entryList acserver.EntryList

	for i := 0; i < 255; i++ {
		x, err := e.GetSection(fmt.Sprintf("CAR_%d", i))

		if err != nil {
			break
		}

		var entrant Entrant

		if err := x.MapTo(&entrant); err != nil {
			return nil, err
		}

		entryList = append(entryList, &acserver.Car{
			Driver: acserver.Driver{
				Name: entrant.Name,
				Team: entrant.Team,
				GUID: entrant.GUID,
			},
			CarID:         acserver.CarID(i),
			Model:         entrant.Model,
			Skin:          entrant.Skin,
			Ballast:       float32(entrant.Ballast),
			Restrictor:    float32(entrant.Restrictor),
			FixedSetup:    entrant.FixedSetup,
			SpectatorMode: uint8(entrant.SpectatorMode),
		})
	}

	return &TempConfig{
		ServerConfig: serverConfig,
		RaceConfig:   eventConfig,
		EntryList:    entryList,
	}, nil
}
