package acserver

import "time"

type Lap struct {
	DriverGUID    string
	DriverName    string
	LapTime       time.Duration
	Cuts          int
	Sectors       []time.Duration
	CompletedTime time.Time
	Tyre          string
	Restrictor    float32
	Ballast       float32
}
