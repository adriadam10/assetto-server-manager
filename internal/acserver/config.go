package acserver

type ServerConfig struct {
	Name                      string        `json:"name" yaml:"name"`
	Password                  string        `json:"password" yaml:"password"`
	AdminPassword             string        `json:"admin_password" yaml:"admin_password"`
	UDPPort                   uint16        `json:"udp_port" yaml:"udp_port"`
	TCPPort                   uint16        `json:"tcp_port" yaml:"tcp_port"`
	HTTPPort                  uint16        `json:"http_port" yaml:"http_port"`
	RegisterToLobby           bool          `json:"register_to_lobby" yaml:"register_to_lobby"`
	ClientSendIntervalInHertz uint8         `json:"client_send_interval_hz" yaml:"client_send_interval_hz"`
	SendBufferSize            int           `json:"send_buffer_size" yaml:"send_buffer_size"`
	ReceiveBufferSize         int           `json:"receive_buffer_size" yaml:"receive_buffer_size"`
	KickQuorum                int           `json:"kick_quorum" yaml:"kick_quorum"`
	VotingQuorum              int           `json:"voting_quorum" yaml:"voting_quorum"`
	VoteDuration              int           `json:"vote_duration" yaml:"vote_duration"`
	BlockListMode             BlockListMode `json:"block_list_mode" yaml:"block_list_mode"`
	NumberOfThreads           int           `json:"number_of_threads" yaml:"number_of_threads"`
	SleepTime                 int           `json:"sleep_time" yaml:"sleep_time"`
	UDPPluginAddress          string        `json:"udp_plugin_address" yaml:"udp_plugin_address"`
	UDPPluginLocalPort        int           `json:"udp_plugin_local_port" yaml:"udp_plugin_local_port"`
	WelcomeMessageFile        string        `json:"welcome_message_file" yaml:"welcome_message_file"`
}

type Assist uint8

const (
	AssistDisabled Assist = 0
	AssistFactory  Assist = 1
	AssistEnabled  Assist = 2
)

type StartRule uint8

const (
	StartRuleCarLockedUntilStart StartRule = 0
	StartRuleTeleportToPits      StartRule = 1
	StartRuleDriveThroughPenalty StartRule = 2
)

type EventConfig struct {
	Cars                      []string `json:"cars" yaml:"cars"`
	Track                     string   `json:"track" yaml:"track"`
	TrackLayout               string   `json:"track_layout" yaml:"track_layout"`
	SunAngle                  float32  `json:"sun_angle" yaml:"sun_angle"`
	LegalTyres                []string `json:"legal_tyres" yaml:"legal_tyres"`
	FuelRate                  float32  `json:"fuel_rate" yaml:"fuel_rate"`
	DamageMultiplier          float32  `json:"damage_multiplier" yaml:"damage_multiplier"`
	TyreWearRate              float32  `json:"tyre_wear_rate" yaml:"tyre_wear_rate"`
	AllowedTyresOut           int16    `json:"allowed_tyres_out" yaml:"allowed_tyres_out"`
	ABSAllowed                Assist   `json:"abs_allowed" yaml:"abs_allowed"`
	TractionControlAllowed    Assist   `json:"traction_control_allowed" yaml:"traction_control_allowed"`
	StabilityControlAllowed   bool     `json:"stability_control_allowed" yaml:"stability_control_allowed"`
	AutoClutchAllowed         bool     `json:"auto_clutch_allowed" yaml:"auto_clutch_allowed"`
	TyreBlanketsAllowed       bool     `json:"tyre_blankets_allowed" yaml:"tyre_blankets_allowed"`
	ForceVirtualMirror        bool     `json:"force_virtual_mirror" yaml:"force_virtual_mirror"`
	ForceOpponentHeadlights   bool     `json:"force_opponent_headlights" yaml:"force_opponent_headlights"`
	RacePitWindowStart        uint16   `json:"race_pit_window_start" yaml:"race_pit_window_start"`
	RacePitWindowEnd          uint16   `json:"race_pit_window_end" yaml:"race_pit_window_end"`
	ReversedGridRacePositions int16    `json:"reversed_grid_race_positions" yaml:"reversed_grid_race_positions"`
	TimeOfDayMultiplier       int      `json:"time_of_day_multiplier" yaml:"time_of_day_multiplier"`
	QualifyMaxWaitPercentage  int      `json:"qualify_max_wait_percentage" yaml:"qualify_max_wait_percentage"`
	RaceGasPenaltyDisabled    bool     `json:"race_gas_penalty_disabled" yaml:"race_gas_penalty_disabled"`
	MaxBallastKilograms       int      `json:"max_ballast_kilograms" yaml:"max_ballast_kilograms"`
	RaceExtraLap              bool     `json:"race_extra_lap" yaml:"race_extra_lap"`
	MaxContactsPerKilometer   uint8    `json:"max_contacts_per_kilometer" yaml:"max_contacts_per_kilometer"`
	ResultScreenTime          uint32   `json:"result_screen_time" yaml:"result_screen_time"`

	PickupModeEnabled bool `json:"pickup_mode_enabled" yaml:"pickup_mode_enabled"`
	LockedEntryList   bool `json:"locked_entry_list" yaml:"locked_entry_list"`
	LoopMode          bool `json:"loop_mode" yaml:"loop_mode"`

	MaxClients   int       `json:"max_clients" yaml:"max_clients"`
	RaceOverTime uint32    `json:"race_over_time" yaml:"race_over_time"`
	StartRule    StartRule `json:"start_rule" yaml:"start_rule"` // @TODO if race is 3 laps or less, enable teleport penalty

	WindBaseSpeedMin       int `json:"wind_base_speed_min" yaml:"wind_base_speed_min"`
	WindBaseSpeedMax       int `json:"wind_base_speed_max" yaml:"wind_base_speed_max"`
	WindBaseDirection      int `json:"wind_base_direction" yaml:"wind_base_direction"`
	WindVariationDirection int `json:"wind_variation_direction" yaml:"wind_variation_direction"`

	DynamicTrack DynamicTrack `json:"dynamic_track" yaml:"dynamic_track"`

	Sessions Sessions         `json:"sessions" yaml:"sessions"`
	Weather  []*WeatherConfig `json:"weather" yaml:"weather"`

	CustomCutsEnabled             bool           `json:"custom_cuts_enabled" yaml:"custom_cuts_enabled"`
	CustomCutsOnlyIfCleanSet      bool           `json:"custom_cuts_only_if_clean_set" yaml:"custom_cuts_only_if_clean_set"`
	CustomCutsIgnoreFirstLap      bool           `json:"custom_cuts_ignore_first_lap" yaml:"custom_cuts_ignore_first_lap"`
	CustomCutsNumWarnings         int            `json:"custom_cuts_num_warnings" yaml:"custom_cuts_num_warnings"`
	CustomCutsPenaltyType         CutPenaltyType `json:"custom_cuts_penalty_type" yaml:"custom_cuts_penalty_type"`
	CustomCutsBoPAmount           float32        `json:"custom_cuts_bop_amount" yaml:"custom_cuts_bop_amount"`
	CustomCutsBoPNumLaps          int            `json:"custom_cuts_bop_num_laps" yaml:"custom_cuts_bop_num_laps"`
	CustomCutsDriveThroughNumLaps int            `json:"custom_cuts_drive_through_num_laps" yaml:"custom_cuts_drive_through_num_laps"`
}

type CutPenaltyType int

const (
	CutPenaltyKick CutPenaltyType = iota
	CutPenaltyBallast
	CutPenaltyRestrictor
	CutPenaltyWarn
	CutPenaltyDriveThrough
)

var CutPenaltyOptions = map[CutPenaltyType]string{
	CutPenaltyKick:         "Kick",
	CutPenaltyBallast:      "Apply Ballast",
	CutPenaltyRestrictor:   "Apply Restrictor",
	CutPenaltyWarn:         "Warn Driver",
	CutPenaltyDriveThrough: "Drive Through",
}

func (c EventConfig) Tyres() map[string]bool {
	tyres := make(map[string]bool)

	for _, tyre := range c.LegalTyres {
		tyres[tyre] = true
	}

	return tyres
}

func (c EventConfig) LobbyTrackName() string {
	track := c.Track

	if c.TrackLayout != "" {
		track += "-" + c.TrackLayout
	}

	return track
}

func (c EventConfig) SessionTypes() []int {
	var types []int

	for _, session := range c.Sessions {
		types = append(types, int(session.SessionType))
	}

	return types
}

func (c EventConfig) SessionDurations() []int {
	var durations []int

	for _, session := range c.Sessions {
		if session.SessionType == SessionTypeRace && session.Laps > 0 {
			durations = append(durations, int(session.Laps))
		} else {
			durations = append(durations, int(session.Time))
		}
	}

	return durations
}

func (c EventConfig) HasSession(t SessionType) bool {
	for _, sess := range c.Sessions {
		if sess.SessionType == t {
			return true
		}
	}

	return false
}

// InGameSessions reports all sessions except SessionTypeBooking.
func (c EventConfig) InGameSessions() []*SessionConfig {
	var out []*SessionConfig

	for _, session := range c.Sessions {
		if session.SessionType != SessionTypeBooking {
			out = append(out, session)
		}
	}

	return out
}

func (c EventConfig) RaceHasLaps() bool {
	for _, session := range c.Sessions {
		if session.SessionType == SessionTypeRace && session.Laps > 0 {
			return true
		}
	}

	return false
}

func (c EventConfig) HasMandatoryPit() bool {
	return c.RacePitWindowStart != 0 && c.RacePitWindowEnd > c.RacePitWindowStart
}

type Sessions []*SessionConfig

func (c EventConfig) HasMultipleRaces() bool {
	return c.ReversedGridRacePositions != 0
}

type OpenRule uint8

const (
	NoJoin                                 OpenRule = 0
	FreeJoin                               OpenRule = 1
	FreeJoinUntil20SecondsBeforeGreenLight OpenRule = 2
)

func (o OpenRule) String() string {
	switch o {
	case NoJoin:
		return "No Join"
	case FreeJoin:
		return "Free Join"
	case FreeJoinUntil20SecondsBeforeGreenLight:
		return "Free Join Until 20 Seconds Before Green Light"
	default:
		return "Unknown Openness"
	}
}
