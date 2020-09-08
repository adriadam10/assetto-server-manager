package acServer

import "fmt"

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
