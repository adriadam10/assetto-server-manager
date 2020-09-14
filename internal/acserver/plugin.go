package acserver

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Plugin interface {
	OnCollisionWithCar(event ClientEvent) error
	OnCollisionWithEnv(event ClientEvent) error
	OnNewSession(newSession SessionInfo) error
	OnNewConnection(car Car) error
	OnConnectionClosed(car Car) error
	OnCarUpdate(carUpdate Car) error
	OnEndSession(sessionFile string) error
	OnVersion(version uint16) error
	OnChat(chat Chat) error
	OnClientLoaded(car Car) error
	OnLapCompleted(carID CarID, lap Lap) error
	OnClientEvent(event ClientEvent) error

	// new
	Init(server *Server, logger Logger) error

	OnSectorCompleted(split Split) error
	OnWeatherChange(weather CurrentWeather) error
}

type multiPlugin struct {
	plugins []Plugin
}

func MultiPlugin(plugins ...Plugin) Plugin {
	return &multiPlugin{plugins: plugins}
}

func (mp *multiPlugin) Init(server *Server, logger Logger) error {
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

func (n nilPlugin) OnCollisionWithCar(event ClientEvent) error {
	return nil
}

func (n nilPlugin) OnCollisionWithEnv(event ClientEvent) error {
	return nil
}

func (n nilPlugin) OnNewSession(newSession SessionInfo) error {
	return nil
}

func (n nilPlugin) OnNewConnection(car Car) error {
	return nil
}

func (n nilPlugin) OnConnectionClosed(car Car) error {
	return nil
}

func (n nilPlugin) OnCarUpdate(carUpdate Car) error {
	return nil
}

func (n nilPlugin) OnEndSession(sessionFile string) error {
	return nil
}

func (n nilPlugin) OnVersion(version uint16) error {
	return nil
}

func (n nilPlugin) OnChat(chat Chat) error {
	return nil
}

func (n nilPlugin) OnClientLoaded(car Car) error {
	return nil
}

func (n nilPlugin) OnLapCompleted(carID CarID, lap Lap) error {
	return nil
}

func (n nilPlugin) OnClientEvent(event ClientEvent) error {
	return nil
}

func (n nilPlugin) Init(server *Server, logger Logger) error {
	return nil
}

func (n nilPlugin) OnSectorCompleted(split Split) error {
	return nil
}

func (n nilPlugin) OnWeatherChange(weather CurrentWeather) error {
	return nil
}
