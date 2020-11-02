package ai

import (
	"image"
	"image/color"
	"io"
	"math"

	"github.com/cj123/ini"
	"github.com/fogleman/gg"

	"justapengu.in/acsm/internal/acserver"
)

func init() {
	ini.PrettyEqual = false
	ini.PrettyFormat = false
}

type TrackMapRenderer struct {
	aiSpline, pitLaneSpline *Spline
	drsZones                map[string]acserver.DRSZone

	offsetX, offsetY float64
	bounds           image.Rectangle
	scaleFactor      float64
}

func NewTrackMapRenderer(aiSpline, pitLaneSpline *Spline, drsZones map[string]acserver.DRSZone) *TrackMapRenderer {
	return &TrackMapRenderer{
		aiSpline:      aiSpline,
		pitLaneSpline: pitLaneSpline,
		drsZones:      drsZones,
	}
}

var (
	pitLaneSplineColor  = color.RGBA{R: 128, G: 128, B: 128, A: 255}
	pitLaneBorderColor  = color.RGBA{R: 104, G: 104, B: 104, A: 255}
	aiSplineColor       = color.White
	aiSplineBorderColor = color.RGBA{R: 40, G: 40, B: 40, A: 255}
	drsZoneColor        = color.RGBA{R: 242, G: 203, B: 10, A: 255}
	drsDetectionColor   = color.RGBA{R: 10, G: 242, B: 87, A: 255}
	startLineColor      = color.RGBA{R: 203, G: 10, B: 242, A: 255}

	pitLanePointSize = 6
	drsWidth         = 6
)

const (
	padding       = 40
	maxTrackWidth = 14
	minTrackWidth = 8
	maxImageWidth = 1600
)

func (t *TrackMapRenderer) point(pt acserver.Vector3F) (float64, float64) {
	return (float64(pt.X) + t.offsetX + padding) / t.scaleFactor, (float64(pt.Z) + t.offsetY + padding) / t.scaleFactor
}

func (t *TrackMapRenderer) calculateAverageWidth() float64 {
	totalWidth := float64(0)

	for _, extra := range t.aiSpline.ExtraPoints {
		totalWidth += float64(extra.SideLeft + extra.SideRight)
	}

	avgWidth := totalWidth / float64(len(t.aiSpline.ExtraPoints))

	if avgWidth > maxTrackWidth {
		avgWidth = maxTrackWidth
	}

	if avgWidth < minTrackWidth {
		avgWidth = minTrackWidth
	}

	sf := t.scaleFactor

	if sf > 1.5 {
		sf /= 1.5
	}

	return avgWidth / sf
}

func (t *TrackMapRenderer) drawPitLane(ctx *gg.Context) {
	ctx.Push()
	for _, point := range t.pitLaneSpline.Points {
		ctx.LineTo(t.point(point.Position))
	}
	ctx.SetColor(pitLaneBorderColor)
	ctx.SetLineWidth(float64(pitLanePointSize) + 4)
	ctx.StrokePreserve()
	ctx.SetColor(pitLaneSplineColor)
	ctx.SetLineWidth(float64(pitLanePointSize))
	ctx.Stroke()
	ctx.Pop()
}

func (t *TrackMapRenderer) drawTrack(ctx *gg.Context, width float64) {
	ctx.Push()
	for _, point := range t.aiSpline.Points {
		ctx.LineTo(t.point(point.Position))
	}
	ctx.SetColor(aiSplineBorderColor)
	ctx.SetLineWidth(width + 4)
	ctx.StrokePreserve()
	ctx.SetColor(aiSplineColor)
	ctx.SetLineWidth(width)
	ctx.Stroke()
	ctx.Pop()
}

func (t *TrackMapRenderer) drawStartFinishLineAndDRSZones(ctx *gg.Context, width float64) {
	totalLen := t.aiSpline.Points[len(t.aiSpline.Points)-1].Length

	for i, point := range t.aiSpline.Points {
		extra := t.aiSpline.ExtraPoints[i]

		forwardVector := extra.ForwardVector.Normalize()

		if forwardVector.Magnitude() == 0 && i+1 < len(t.aiSpline.Points) {
			// build the forward vector from the next track point
			nextPoint := t.aiSpline.Points[i+1]
			forwardVector = acserver.Vector3F{
				X: nextPoint.Position.X - point.Position.X,
				Y: nextPoint.Position.Y - point.Position.Y,
				Z: nextPoint.Position.Z - point.Position.Z,
			}.Normalize()
		}

		x, y := t.point(point.Position)

		if i == 0 {
			// start line
			drawPerpendicularLine(ctx, forwardVector, x, y, startLineColor, int(width*3), drsWidth)
		}

		isZoneStart := false
		isZoneEnd := false
		isDetectionPoint := false

		pos := point.Length / totalLen

		var nextPos float32

		if i+1 < len(t.aiSpline.Points) {
			nextPos = t.aiSpline.Points[i+1].Length / totalLen
		} else {
			nextPos = 1.0
		}

		for _, zone := range t.drsZones {
			if zone.Start == zone.End || math.Abs(float64(zone.End-zone.Start)) <= 0.001 || zone.Detection > zone.Start && zone.Detection > zone.End {
				continue
			}

			if zone.Detection >= pos && zone.Detection < nextPos {
				isDetectionPoint = true
			}

			if zone.Start >= pos && zone.Start < nextPos {
				isZoneStart = true
			}

			if zone.End >= pos && zone.End < nextPos {
				isZoneEnd = true
			}
		}

		if isDetectionPoint {
			drawPerpendicularLine(ctx, forwardVector, x, y, drsDetectionColor, int(width*2), drsWidth)
		}

		if isZoneStart {
			drawPerpendicularLine(ctx, forwardVector, x, y, drsZoneColor, int(width*2), drsWidth)
		}

		if isZoneEnd {
			drawPerpendicularLine(ctx, forwardVector, x, y, drsZoneColor, int(width*2), drsWidth)
		}
	}
}

func (t *TrackMapRenderer) Render(w io.Writer) (*TrackMapData, error) {
	t.bounds, t.offsetX, t.offsetY = t.Rect()
	img := image.NewRGBA(t.bounds)
	ctx := gg.NewContextForRGBA(img)

	avgWidth := t.calculateAverageWidth()

	t.drawPitLane(ctx)
	t.drawTrack(ctx, avgWidth)
	t.drawStartFinishLineAndDRSZones(ctx, avgWidth)

	data := &TrackMapData{
		Width:       float64(t.bounds.Dx()),
		Height:      float64(t.bounds.Dy()),
		Margin:      10,
		ScaleFactor: t.scaleFactor,
		OffsetX:     t.offsetX + padding,
		OffsetZ:     t.offsetY + padding,
		DrawingSize: 10, // idk what this is for, but server manager doesn't need it.
	}

	return data, ctx.EncodePNG(w)
}

func drawPerpendicularLine(ctx *gg.Context, forwardVector acserver.Vector3F, x, y float64, color color.Color, length int, width int) {
	if forwardVector.Magnitude() == 0 {
		return
	}

	perpendicularVector := acserver.Vector3F{X: forwardVector.Z, Z: -forwardVector.X}
	min := perpendicularVector.Mul(float32(length) / 2)

	ctx.Push()
	ctx.LineTo(x+float64(min.X), y+float64(min.Z))
	ctx.LineTo(x-float64(min.X), y-float64(min.Z))
	ctx.SetColor(aiSplineBorderColor)
	ctx.SetLineWidth(float64(width) + 4)
	ctx.StrokePreserve()
	ctx.SetColor(color)
	ctx.SetLineWidth(float64(width))
	ctx.Stroke()
	ctx.Pop()
}

func (t *TrackMapRenderer) Rect() (rect image.Rectangle, offsetX, offsetY float64) {
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

	if minX != 0 {
		offsetX = -float64(minX)
		maxX -= minX
		minX = 0
	}

	if minY != 0 {
		offsetY = -float64(minY)
		maxY -= minY
		minY = 0
	}

	dx := maxX - minX

	if dx > maxImageWidth {
		t.scaleFactor = math.Round(float64(dx) / maxImageWidth)

		minX /= float32(t.scaleFactor)
		maxX /= float32(t.scaleFactor)
		minY /= float32(t.scaleFactor)
		maxY /= float32(t.scaleFactor)
	} else {
		t.scaleFactor = 1
	}

	maxX += padding * 2
	maxY += padding * 2

	return image.Rect(int(minX), int(minY), int(maxX), int(maxY)), offsetX, offsetY
}

type TrackMapData struct {
	Width       float64 `ini:"WIDTH" json:"width"`
	Height      float64 `ini:"HEIGHT" json:"height"`
	Margin      float64 `ini:"MARGIN" json:"margin"`
	ScaleFactor float64 `ini:"SCALE_FACTOR" json:"scale_factor"`
	OffsetX     float64 `ini:"X_OFFSET" json:"offset_x"`
	OffsetZ     float64 `ini:"Z_OFFSET" json:"offset_y"`
	DrawingSize float64 `ini:"DRAWING_SIZE" json:"drawing_size"`
}

func (tmd *TrackMapData) Save(path string) error {
	i := ini.Empty()

	sec, err := i.NewSection("PARAMETERS")

	if err != nil {
		return err
	}

	if err := sec.ReflectFrom(tmd); err != nil {
		return err
	}

	return i.SaveTo(path)
}
