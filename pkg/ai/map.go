package ai

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
)

type TrackMapRenderer struct {
	aiSpline, pitLaneSpline *Spline
}

func NewTrackMapRenderer(aiSpline, pitLaneSpline *Spline) *TrackMapRenderer {
	return &TrackMapRenderer{
		aiSpline:      aiSpline,
		pitLaneSpline: pitLaneSpline,
	}
}

var (
	pitLaneSplineColor = color.RGBA{R: 128, G: 128, B: 128, A: 255}
	aiSplineColor      = color.White

	pointSize = 8
)

func (t *TrackMapRenderer) Render(w io.Writer) error {
	bounds := t.Rect()
	img := image.NewRGBA(bounds)

	for _, point := range t.pitLaneSpline.Points {
		draw.Draw(img, bounds, newCircle(image.Pt(int(point.Position.X), int(point.Position.Z)), pointSize, pitLaneSplineColor), image.Pt(0, 0), draw.Over)
	}

	for _, point := range t.aiSpline.Points {
		draw.Draw(img, bounds, newCircle(image.Pt(int(point.Position.X), int(point.Position.Z)), pointSize, aiSplineColor), image.Pt(0, 0), draw.Over)
	}

	return png.Encode(w, img)
}

func (t *TrackMapRenderer) Rect() image.Rectangle {
	minX, minY := t.aiSpline.Min()
	maxX, maxY := t.aiSpline.Max()

	if t.pitLaneSpline != nil {
		pitLaneMinX, pitLaneMinY := t.pitLaneSpline.Min()
		pitLaneMaxX, pitLaneMaxY := t.pitLaneSpline.Max()

		if pitLaneMinX < minX {
			minX = pitLaneMinX
		}

		if pitLaneMinY < minY {
			minY = pitLaneMinY
		}

		if pitLaneMaxX > maxX {
			maxX = pitLaneMaxX
		}

		if pitLaneMaxY > maxY {
			maxY = pitLaneMaxY
		}
	}

	return image.Rect(int(minX), int(minY), int(maxX), int(maxY))
}

func (t *TrackMapRenderer) Dimensions() (int, int) {
	rect := t.Rect()

	return rect.Dx(), rect.Dy()
}

type circle struct {
	p     image.Point
	r     int
	color color.Color
}

func newCircle(point image.Point, radius int, color color.Color) *circle {
	return &circle{
		p:     point,
		r:     radius,
		color: color,
	}
}

func (c *circle) ColorModel() color.Model {
	return color.AlphaModel
}

func (c *circle) Bounds() image.Rectangle {
	return image.Rect(c.p.X-c.r, c.p.Y-c.r, c.p.X+c.r, c.p.Y+c.r)
}

func (c *circle) At(x, y int) color.Color {
	xx, yy, rr := float64(x-c.p.X)+0.5, float64(y-c.p.Y)+0.5, float64(c.r)

	if xx*xx+yy*yy < rr*rr {
		return c.color
	}

	return color.RGBA{}
}
