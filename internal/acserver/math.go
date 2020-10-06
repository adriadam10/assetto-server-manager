package acserver

import (
	"math"
)

type Vector3F struct {
	X float32
	Y float32
	Z float32
}

func (v Vector3F) DistanceTo(b Vector3F) float64 {
	x := math.Pow(float64(b.X-v.X), 2)
	y := math.Pow(float64(b.Y-v.Y), 2)
	z := math.Pow(float64(b.Z-v.Z), 2)

	return math.Sqrt(x + y + z)
}

func (v Vector3F) Add(other Vector3F) Vector3F {
	return Vector3F{
		X: v.X + other.X,
		Y: v.Y + other.Y,
		Z: v.Z + other.Z,
	}
}

func (v Vector3F) Sub(other Vector3F) Vector3F {
	return Vector3F{
		X: v.X - other.X,
		Y: v.Y - other.Y,
		Z: v.Z - other.Z,
	}
}

func (v Vector3F) Mul(val float32) Vector3F {
	return Vector3F{
		X: v.X * val,
		Y: v.Y * val,
		Z: v.Z * val,
	}
}

func (v Vector3F) Dot(ov Vector3F) float32 {
	return v.X*ov.X + v.Y*ov.Y + v.Z*ov.Z
}
