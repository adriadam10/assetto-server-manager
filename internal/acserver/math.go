package acserver

import "math"

type Vector3F struct {
	X float32
	Y float32
	Z float32
}

func (a Vector3F) DistanceTo(b Vector3F) float64 {
	x := math.Pow(float64(b.X-a.X), 2)
	y := math.Pow(float64(b.Y-a.Y), 2)
	z := math.Pow(float64(b.Z-a.Z), 2)

	return math.Sqrt(x + y + z)
}
