package acserver

import (
	"time"
)

type Plugin interface {
	Init(server ServerPlugin, logger Logger) error
	Shutdown() error

	OnVersion(version uint16) error
	OnNewSession(newSession SessionInfo) error
	OnWeatherChange(weather CurrentWeather) error
	OnEndSession(sessionFile string) error

	OnNewConnection(car CarInfo) error
	OnClientLoaded(car CarInfo) error
	OnSectorCompleted(car CarInfo, split Split) error
	OnLapCompleted(carID CarID, lap Lap) error
	OnCarUpdate(carUpdate CarInfo) error
	OnTyreChange(car CarInfo, tyres string) error

	OnClientEvent(event ClientEvent) error
	OnCollisionWithCar(event ClientEvent) error
	OnCollisionWithEnv(event ClientEvent) error

	OnChat(chat Chat) error
	OnConnectionClosed(car CarInfo) error

	// SortLeaderboard is called whenever the Leaderboard is built.
	SortLeaderboard(sessionType SessionType, leaderboard []*LeaderboardLine)
}

type ServerPlugin interface {
	GetCarInfo(id CarID) (CarInfo, error)
	CarIsConnected(id CarID) bool
	GetSessionInfo() SessionInfo
	GetEventConfig() EventConfig

	// AddDriver adds a driver to an existing entrant on the entry list, so long as there is an available slot
	AddDriver(name, team, guid, model string) error

	// SendChat sends a chat message to a car on the server.
	// Note that setting rateLimit to true will block until sending completes.
	SendChat(message string, from, to CarID, rateLimit bool) error

	// BroadcastChat sends a chat message to all cars on the server.
	// Note that setting rateLimit to true will block until sending completes.
	BroadcastChat(message string, from CarID, rateLimit bool)

	UpdateBoP(carIDToUpdate CarID, ballast, restrictor float32) error
	KickUser(carIDToKick CarID, reason KickReason) error

	NextSession()
	RestartSession()
	SetCurrentSession(index uint8, config *SessionConfig)

	AdminCommand(command string) error

	GetLeaderboard() []*LeaderboardLine

	// Fixed setups can be sent mid session
	SendSetup(overrideValues map[string]float32, carID CarID) error

	SetUpdateInterval(interval time.Duration)
}

type multiPlugin struct {
	plugins []Plugin
}

func MultiPlugin(plugins ...Plugin) Plugin {
	return &multiPlugin{plugins: plugins}
}

func (mp *multiPlugin) Init(server ServerPlugin, logger Logger) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.Init(server, logger))
	}

	return errs.Err()
}

func (mp *multiPlugin) Shutdown() error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.Shutdown())
	}

	return errs.Err()
}

func (mp *multiPlugin) OnCollisionWithCar(event ClientEvent) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnCollisionWithCar(event))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnCollisionWithEnv(event ClientEvent) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnCollisionWithEnv(event))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnNewSession(newSession SessionInfo) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnNewSession(newSession))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnNewConnection(car CarInfo) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnNewConnection(car))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnConnectionClosed(car CarInfo) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnConnectionClosed(car))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnCarUpdate(carUpdate CarInfo) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnCarUpdate(carUpdate))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnTyreChange(car CarInfo, tyres string) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnTyreChange(car, tyres))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnEndSession(sessionFile string) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnEndSession(sessionFile))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnVersion(version uint16) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnVersion(version))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnChat(chat Chat) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnChat(chat))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnClientLoaded(car CarInfo) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnClientLoaded(car))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnLapCompleted(carID CarID, lap Lap) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnLapCompleted(carID, lap))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnClientEvent(event ClientEvent) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnClientEvent(event))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnSectorCompleted(car CarInfo, split Split) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnSectorCompleted(car, split))
	}

	return errs.Err()
}

func (mp *multiPlugin) OnWeatherChange(weather CurrentWeather) error {
	errs := make(groupedError, 0)

	for _, plugin := range mp.plugins {
		errs = append(errs, plugin.OnWeatherChange(weather))
	}

	return errs.Err()
}

func (mp *multiPlugin) SortLeaderboard(sessionType SessionType, leaderboard []*LeaderboardLine) {
	for _, plugin := range mp.plugins {
		plugin.SortLeaderboard(sessionType, leaderboard)
	}
}

type nilPlugin struct{}

func (n nilPlugin) OnCollisionWithCar(_ ClientEvent) error {
	return nil
}

func (n nilPlugin) OnCollisionWithEnv(_ ClientEvent) error {
	return nil
}

func (n nilPlugin) OnNewSession(_ SessionInfo) error {
	return nil
}

func (n nilPlugin) OnNewConnection(_ CarInfo) error {
	return nil
}

func (n nilPlugin) OnConnectionClosed(_ CarInfo) error {
	return nil
}

func (n nilPlugin) OnCarUpdate(_ CarInfo) error {
	return nil
}

func (n nilPlugin) OnTyreChange(car CarInfo, tyres string) error {
	return nil
}

func (n nilPlugin) OnEndSession(_ string) error {
	return nil
}

func (n nilPlugin) OnVersion(_ uint16) error {
	return nil
}

func (n nilPlugin) OnChat(_ Chat) error {
	return nil
}

func (n nilPlugin) OnClientLoaded(_ CarInfo) error {
	return nil
}

func (n nilPlugin) OnLapCompleted(_ CarID, _ Lap) error {
	return nil
}

func (n nilPlugin) OnClientEvent(_ ClientEvent) error {
	return nil
}

func (n nilPlugin) Init(_ ServerPlugin, _ Logger) error {
	return nil
}

func (n nilPlugin) Shutdown() error {
	return nil
}

func (n nilPlugin) OnSectorCompleted(_ CarInfo, _ Split) error {
	return nil
}

func (n nilPlugin) OnWeatherChange(_ CurrentWeather) error {
	return nil
}

func (n nilPlugin) SortLeaderboard(_ SessionType, _ []*LeaderboardLine) {

}
