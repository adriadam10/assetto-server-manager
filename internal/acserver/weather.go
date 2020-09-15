package acserver

import (
	"fmt"
	"math/rand"
)

type WeatherConfig struct {
	Graphics               string `json:"graphics" yaml:"graphics"`
	Duration               int64  `json:"duration" yaml:"duration"`
	BaseTemperatureAmbient int    `json:"base_temperature_ambient" yaml:"base_temperature_ambient"`
	BaseTemperatureRoad    int    `json:"base_temperature_road" yaml:"base_temperature_road"`
	VariationAmbient       int    `json:"variation_ambient" yaml:"variation_ambient"`
	VariationRoad          int    `json:"variation_road" help:"variation_road"`
	WindBaseSpeedMin       int    `json:"wind_base_speed_min" yaml:"wind_base_speed_min"`
	WindBaseSpeedMax       int    `json:"wind_base_speed_max" yaml:"wind_base_speed_max"`
	WindBaseDirection      int    `json:"wind_base_direction" yaml:"wind_base_direction"`
	WindVariationDirection int    `json:"wind_variation_direction" yaml:"wind_variation_direction"`
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

	currentWeather *CurrentWeather
}

func NewWeatherManager(state *ServerState, plugin Plugin, logger Logger) *WeatherManager {
	return &WeatherManager{
		state:  state,
		plugin: plugin,
		logger: logger,
	}
}

func (wm *WeatherManager) ChangeWeather(weatherConfig *WeatherConfig) {
	ambient, road := wm.calculateTemperatures(weatherConfig)
	windSpeed, windDirection := wm.calculateWind(weatherConfig)

	wm.currentWeather = &CurrentWeather{
		Ambient:       ambient,
		Road:          road,
		GraphicsName:  weatherConfig.Graphics,
		WindSpeed:     windSpeed,
		WindDirection: windDirection,
	}

	for _, car := range wm.state.entryList {
		if !car.IsConnected() {
			continue
		}

		if err := wm.SendWeather(car); err != nil {
			wm.logger.WithError(err).Errorf("Could not send weather to car: %s", car.String())
		}
	}

	go func() {
		err := wm.plugin.OnWeatherChange(*wm.currentWeather)

		if err != nil {
			wm.logger.WithError(err).Error("On weather change plugin returned an error")
		}
	}()
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
	wm.logger.Infof("Sending Weather (%s), to entrant: %s", wm.currentWeather.String(), entrant.String())

	bw := NewPacket(nil)
	bw.Write(TCPSendWeather)
	bw.Write(wm.currentWeather.Ambient)
	bw.Write(wm.currentWeather.Road)
	bw.WriteUTF32String(wm.currentWeather.GraphicsName)
	bw.Write(wm.currentWeather.WindSpeed)
	bw.Write(wm.currentWeather.WindDirection)

	return bw.WriteTCP(entrant.Connection.tcpConn)
}
