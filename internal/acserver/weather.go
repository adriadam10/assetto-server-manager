package acserver

import (
	"fmt"
	"math/rand"
	"sync"
)

type WeatherConfig struct {
	Graphics               string        `json:"graphics" yaml:"graphics"`
	Duration               int64         `json:"duration" yaml:"duration"`
	Sessions               []SessionType `json:"sessions" yaml:"sessions"`
	BaseTemperatureAmbient int           `json:"base_temperature_ambient" yaml:"base_temperature_ambient"`
	BaseTemperatureRoad    int           `json:"base_temperature_road" yaml:"base_temperature_road"`
	VariationAmbient       int           `json:"variation_ambient" yaml:"variation_ambient"`
	VariationRoad          int           `json:"variation_road" help:"variation_road"`
	WindBaseSpeedMin       int           `json:"wind_base_speed_min" yaml:"wind_base_speed_min"`
	WindBaseSpeedMax       int           `json:"wind_base_speed_max" yaml:"wind_base_speed_max"`
	WindBaseDirection      int           `json:"wind_base_direction" yaml:"wind_base_direction"`
	WindVariationDirection int           `json:"wind_variation_direction" yaml:"wind_variation_direction"`
}

func (w *WeatherConfig) AppliesToSession(s SessionType) bool {
	if len(w.Sessions) == 0 {
		return true
	}

	for _, sess := range w.Sessions {
		if s == sess {
			return true
		}
	}

	return false
}

type CurrentWeather struct {
	Ambient       uint8
	Road          uint8
	GraphicsName  string
	WindSpeed     int16
	WindDirection int16
}

func (c CurrentWeather) String() string {
	return fmt.Sprintf("%s, %d°/%d° ambient/road, %d/%d wind speed/direction", c.GraphicsName, c.Ambient, c.Road, c.WindSpeed, c.WindDirection)
}

type WeatherManager struct {
	state  *ServerState
	plugin Plugin
	logger Logger

	mutex               sync.Mutex
	currentWeather      *CurrentWeather
	nextWeatherUpdate   int64
	currentWeatherIndex int
	weatherProgression  bool

	sunAngle               float32
	sunAngleUpdateInterval int64
	lastSunUpdate          int64
}

func NewWeatherManager(state *ServerState, plugin Plugin, logger Logger) *WeatherManager {
	sunAngleUpdateInterval := int64(60000)

	if state.raceConfig.TimeOfDayMultiplier == 0 {
		state.raceConfig.TimeOfDayMultiplier = 1
	}

	if state.raceConfig.TimeOfDayMultiplier > 0 {
		// @TODO what is the performance impact of this? Turn off when CSP/Sol enabled (probably)
		sunAngleUpdateInterval = int64(float32(60000) / float32(state.raceConfig.TimeOfDayMultiplier))
	}

	return &WeatherManager{
		state:                  state,
		plugin:                 plugin,
		logger:                 logger,
		sunAngle:               state.raceConfig.SunAngle,
		sunAngleUpdateInterval: sunAngleUpdateInterval,
	}
}

func (wm *WeatherManager) GetCurrentWeather() CurrentWeather {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	if wm.currentWeather == nil {
		return CurrentWeather{}
	}

	return *wm.currentWeather
}

func (wm *WeatherManager) ChangeWeather(weatherConfig *WeatherConfig, weatherUpdateOffsetSeconds int64) {
	ambient, road := wm.calculateTemperatures(weatherConfig)
	windSpeed, windDirection := wm.calculateWind(weatherConfig)

	wm.mutex.Lock()
	wm.currentWeather = &CurrentWeather{
		Ambient:       ambient,
		Road:          road,
		GraphicsName:  weatherConfig.Graphics,
		WindSpeed:     windSpeed,
		WindDirection: windDirection,
	}
	wm.logger.Infof("Changed weather to: %s", wm.currentWeather.String())

	if weatherConfig.Duration > 0 {
		wm.nextWeatherUpdate = currentTimeMillisecond() + (weatherUpdateOffsetSeconds * 1000) + (weatherConfig.Duration * 60000)
	} else {
		wm.logger.Infof("Weather progression completed.")
		wm.weatherProgression = false
	}
	wm.mutex.Unlock()

	for _, car := range wm.state.entryList {
		if !car.IsConnected() {
			continue
		}

		if err := wm.SendWeather(car); err != nil {
			wm.logger.WithError(err).Errorf("Could not send weather to car: %s", car.String())
		}
	}

	err := wm.plugin.OnWeatherChange(wm.GetCurrentWeather())

	if err != nil {
		wm.logger.WithError(err).Error("On weather change plugin returned an error")
	}
}

func (wm *WeatherManager) calculateTemperatures(weatherConfig *WeatherConfig) (ambient, road uint8) {
	var ambientModifier int

	if weatherConfig.VariationAmbient > 0 {
		ambientModifier = rand.Intn(weatherConfig.VariationAmbient*2) - weatherConfig.VariationAmbient
	}

	ambient = uint8(weatherConfig.BaseTemperatureAmbient + ambientModifier)

	var roadModifier int

	if weatherConfig.VariationRoad > 0 {
		roadModifier = rand.Intn(weatherConfig.VariationRoad*2) - weatherConfig.VariationRoad
	}

	road = uint8(int(ambient) + weatherConfig.BaseTemperatureRoad + roadModifier)

	return ambient, road
}

func (wm *WeatherManager) calculateWind(weatherConfig *WeatherConfig) (speed, direction int16) {
	windRange := weatherConfig.WindBaseSpeedMax - weatherConfig.WindBaseSpeedMin

	var windModifier int

	if windRange > 0 {
		windModifier = rand.Intn(windRange)
	}

	speed = int16(weatherConfig.WindBaseSpeedMin + windModifier)

	if speed > 40 {
		speed = 40
	}

	var directionModifier int

	if weatherConfig.WindVariationDirection > 0 {
		directionModifier = rand.Intn(weatherConfig.WindVariationDirection*2) - weatherConfig.WindVariationDirection
	}

	direction = int16(weatherConfig.WindBaseDirection + directionModifier)

	return speed, direction
}

func (wm *WeatherManager) SendWeather(entrant *Car) error {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()
	wm.logger.Infof("Sending weather (%s), to entrant: %s", wm.currentWeather.String(), entrant.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendWeather)
	bw.Write(wm.currentWeather.Ambient)
	bw.Write(wm.currentWeather.Road)
	bw.WriteUTF32String(wm.currentWeather.GraphicsName)
	bw.Write(wm.currentWeather.WindSpeed)
	bw.Write(wm.currentWeather.WindDirection)

	return bw.WriteTCP(entrant.Connection.tcpConn)
}

func (wm *WeatherManager) SendSunAngle() {
	wm.logger.Debugf("Broadcasting Sun Angle (%.2f)", wm.sunAngle)

	bw := NewPacket(nil)
	bw.Write(TCPSendSunAngle)
	bw.Write(wm.sunAngle)

	wm.state.BroadcastAllTCP(bw)
}

func (wm *WeatherManager) SendSunAngleToCar(car *Car) error {
	bw := NewPacket(nil)
	bw.Write(TCPSendSunAngle)
	bw.Write(wm.sunAngle)

	return bw.WriteTCP(car.Connection.tcpConn)
}

func (wm *WeatherManager) OnNewSession(session SessionConfig) {
	wm.mutex.Lock()
	wm.currentWeatherIndex = 0
	wm.weatherProgression = false
	wm.nextWeatherUpdate = 0
	wm.sunAngle = wm.state.raceConfig.SunAngle
	wm.mutex.Unlock()

	sessionWeatherIndex := -1

	allWeathersForSessionHaveZeroDuration := true
	var weathersForCurrentSession []*WeatherConfig

	for _, weather := range wm.state.raceConfig.Weather {
		if weather.AppliesToSession(session.SessionType) {
			weathersForCurrentSession = append(weathersForCurrentSession, weather)
			allWeathersForSessionHaveZeroDuration = allWeathersForSessionHaveZeroDuration && weather.Duration == 0
		}
	}

	if allWeathersForSessionHaveZeroDuration {
		if len(weathersForCurrentSession) > 0 {
			wm.logger.Infof("Randomly choosing a weather for the current session")
			randomWeather := weathersForCurrentSession[rand.Intn(len(weathersForCurrentSession))]

			// find the right index in our original list for the randomly selected weather
			for weatherIndex, weather := range wm.state.raceConfig.Weather {
				if weather == randomWeather {
					sessionWeatherIndex = weatherIndex
				}
			}
		}
	} else {
		for weatherIndex, weather := range wm.state.raceConfig.Weather {
			if weather.AppliesToSession(session.SessionType) && sessionWeatherIndex < 0 {
				sessionWeatherIndex = weatherIndex
				continue
			}

			if weather.AppliesToSession(session.SessionType) && sessionWeatherIndex >= 0 {
				wm.logger.Debugf("Session has multiple weathers. Enabling weather progression.")
				wm.weatherProgression = true
				break
			}
		}
	}

	if sessionWeatherIndex >= 0 {
		wm.mutex.Lock()
		wm.currentWeatherIndex = sessionWeatherIndex
		wm.mutex.Unlock()

		wm.ChangeWeather(wm.state.raceConfig.Weather[wm.currentWeatherIndex], int64(session.WaitTime))
	} else if sessionWeatherIndex < 0 {
		wm.logger.Debugf("No weather defined for session. Falling back to sensible defaults.")

		wm.ChangeWeather(&WeatherConfig{
			Graphics:               "3_clear",
			Duration:               0,
			BaseTemperatureAmbient: 26,
			BaseTemperatureRoad:    11,
			VariationAmbient:       1,
			VariationRoad:          1,
			WindBaseSpeedMin:       3,
			WindBaseSpeedMax:       15,
			WindBaseDirection:      30,
			WindVariationDirection: 15,
		}, int64(session.WaitTime))
	}
}

const (
	minSunAngle = -80
	maxSunAngle = 80
)

func (wm *WeatherManager) Step(currentTime int64, currentSession SessionConfig) {
	// @TODO (improvement) at 1x this loses between 0.5 and 1s evey 60s
	if currentTime-wm.lastSunUpdate > wm.sunAngleUpdateInterval || wm.lastSunUpdate == 0 {
		// @TODO with CSP exceeding -80 and 80 works fine, and you can loop!
		wm.sunAngle = wm.state.raceConfig.SunAngle + float32(wm.state.raceConfig.TimeOfDayMultiplier)*(0.0044*(float32(currentTime)/1000.0))

		if wm.sunAngle < minSunAngle {
			wm.sunAngle = minSunAngle
		} else if wm.sunAngle > maxSunAngle {
			wm.sunAngle = maxSunAngle
		}

		wm.SendSunAngle()

		wm.lastSunUpdate = currentTime
	}

	if wm.ShouldProgressToNextWeather(currentTime) {
		wm.NextWeather(currentSession)
	}
}

func (wm *WeatherManager) ShouldProgressToNextWeather(currentTime int64) bool {
	wm.mutex.Lock()
	defer wm.mutex.Unlock()

	return wm.weatherProgression && wm.nextWeatherUpdate < currentTime
}

func (wm *WeatherManager) NextWeather(currentSession SessionConfig) {
	nextWeatherIndex := -1

	for i := wm.currentWeatherIndex + 1; i < len(wm.state.raceConfig.Weather); i++ {
		weather := wm.state.raceConfig.Weather[i]

		if weather.AppliesToSession(currentSession.SessionType) {
			nextWeatherIndex = i
			break
		}
	}

	if nextWeatherIndex <= 0 {
		// see if we can loop back
		for weatherIndex, weather := range wm.state.raceConfig.Weather {
			if weather.AppliesToSession(currentSession.SessionType) {
				nextWeatherIndex = weatherIndex
				break
			}
		}
	}

	if nextWeatherIndex >= 0 {
		nextWeather := wm.state.raceConfig.Weather[nextWeatherIndex]

		wm.currentWeatherIndex = nextWeatherIndex
		wm.ChangeWeather(nextWeather, 0)
	} else {
		wm.logger.Infof("Weather progression completed.")
		wm.weatherProgression = false
	}
}
