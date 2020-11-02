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

func (v Vector3F) Magnitude() float64 {
	return math.Sqrt(math.Pow(float64(v.X), 2) + math.Pow(float64(v.Y), 2) + math.Pow(float64(v.Z), 2))
}

func (v Vector3F) Cross(ov Vector3F) Vector3F {
	return Vector3F{
		v.Y*ov.Z - v.Z*ov.Y,
		v.Z*ov.X - v.X*ov.Z,
		v.X*ov.Y - v.Y*ov.X,
	}
}

// Norm2 returns the square of the norm.
func (v Vector3F) Norm2() float32 { return v.Dot(v) }

// Normalize returns a unit vector in the same direction as v.
func (v Vector3F) Normalize() Vector3F {
	n2 := v.Norm2()
	if n2 == 0 {
		return Vector3F{0, 0, 0}
	}
	return v.Mul(float32(1 / math.Sqrt(float64(n2))))
}
