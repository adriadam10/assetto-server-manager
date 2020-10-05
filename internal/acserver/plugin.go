package acserver

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

type Plugin interface {
	Init(server ServerPlugin, logger Logger) error

	OnVersion(version uint16) error
	OnNewSession(newSession SessionInfo) error
	OnWeatherChange(weather CurrentWeather) error
	OnEndSession(sessionFile string) error

	OnNewConnection(car Car) error
	OnClientLoaded(car Car) error
	OnSectorCompleted(split Split) error
	OnLapCompleted(carID CarID, lap Lap) error
	OnCarUpdate(carUpdate Car) error
	OnTyreChange(car Car, tyres string) error

	OnClientEvent(event ClientEvent) error
	OnCollisionWithCar(event ClientEvent) error
	OnCollisionWithEnv(event ClientEvent) error

	OnChat(chat Chat) error
	OnConnectionClosed(car Car) error
}

type ServerPlugin interface {
	GetCarInfo(id CarID) (Car, error)
	GetSessionInfo() SessionInfo
	GetEventConfig() EventConfig

	SendChat(message string, from, to CarID, rateLimit bool) error
	BroadcastChat(message string, from CarID, rateLimit bool)

	UpdateBoP(carIDToUpdate CarID, ballast, restrictor float32) error
	KickUser(carIDToKick CarID, reason KickReason) error

	NextSession()
	RestartSession()
	SetCurrentSession(index uint8, config *SessionConfig)

	AdminCommand(command string) error

	GetLeaderboard() []*LeaderboardLine

	SetUpdateInterval(interval time.Duration)
}

type multiPlugin struct {
	plugins []Plugin
}

func MultiPlugin(plugins ...Plugin) Plugin {
	return &multiPlugin{plugins: plugins}
}

func (mp *multiPlugin) Init(server ServerPlugin, logger Logger) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.Init(server, logger)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnCollisionWithCar(event ClientEvent) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnCollisionWithCar(event)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnCollisionWithEnv(event ClientEvent) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnCollisionWithEnv(event)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnNewSession(newSession SessionInfo) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnNewSession(newSession)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnNewConnection(car Car) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnNewConnection(car)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnConnectionClosed(car Car) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnConnectionClosed(car)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnCarUpdate(carUpdate Car) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnCarUpdate(carUpdate)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnTyreChange(car Car, tyres string) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnTyreChange(car, tyres)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnEndSession(sessionFile string) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnEndSession(sessionFile)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnVersion(version uint16) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnVersion(version)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnChat(chat Chat) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnChat(chat)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnClientLoaded(car Car) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnClientLoaded(car)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnLapCompleted(carID CarID, lap Lap) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnLapCompleted(carID, lap)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnClientEvent(event ClientEvent) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnClientEvent(event)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnSectorCompleted(split Split) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnSectorCompleted(split)
		})
	}

	return g.Wait()
}

func (mp *multiPlugin) OnWeatherChange(weather CurrentWeather) error {
	g, _ := errgroup.WithContext(context.Background())

	for _, plugin := range mp.plugins {
		plugin := plugin
		g.Go(func() error {
			return plugin.OnWeatherChange(weather)
		})
	}

	return g.Wait()
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

func (n nilPlugin) OnNewConnection(_ Car) error {
	return nil
}

func (n nilPlugin) OnConnectionClosed(_ Car) error {
	return nil
}

func (n nilPlugin) OnCarUpdate(_ Car) error {
	return nil
}

func (n nilPlugin) OnTyreChange(car Car, tyres string) error {
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

func (n nilPlugin) OnClientLoaded(_ Car) error {
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

func (n nilPlugin) OnSectorCompleted(_ Split) error {
	return nil
}

func (n nilPlugin) OnWeatherChange(_ CurrentWeather) error {
	return nil
}
