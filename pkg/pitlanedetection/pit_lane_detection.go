package pitlanedetection

import (
	"errors"
	"math"
	"path/filepath"
	"sort"
	"time"

	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/ai"
	"justapengu.in/acsm/pkg/udp"
)

type PitLane struct {
	Cars []*PitLaneCar

	PitLaneSpline *ai.Spline
	TrackSpline   *ai.Spline

	PitLanePoints, TrackPoints []acserver.Vector3F

	AveragePitLaneTime time.Duration

	PitLaneCapable bool
}

type PitLaneCar struct {
	ID uint8

	IsInPits bool
}

func (p *PitLane) UpdateCar(carID uint8, isInPits bool) {
	for _, car := range p.Cars {
		if car.ID == carID {
			car.IsInPits = isInPits
			return
		}
	}

	p.Cars = append(p.Cars, &PitLaneCar{
		ID:       carID,
		IsInPits: isInPits,
	})
}

func NewSharedPitLane(serverInstallPath, track, layout string, distance, maxDistance float64, maxSpeed float32) (*PitLane, error) {
	var pitLanePoints []acserver.Vector3F
	var trackPoints []acserver.Vector3F

	trackSpline, err := ai.ReadSpline(filepath.Join(serverInstallPath, "content", "tracks", track, layout, "ai", "fast_lane.ai"))

	if err != nil {
		return nil, err
	}

	pitLaneSpline, err := ai.ReadPitLaneSpline(
		filepath.Join(serverInstallPath, "content", "tracks", track, layout, "ai"),
		trackSpline,
		maxSpeed,
		distance,
		maxDistance,
	)

	if err != nil {
		return nil, err
	}

	for _, point := range pitLaneSpline.Points {
		pitLanePoints = append(pitLanePoints, point.Position)
	}

	for _, point := range trackSpline.Points {
		trackPoints = append(trackPoints, point.Position)
	}

	if len(pitLanePoints) == 0 {
		return nil, errors.New("plugins: no pitlane points found")
	}

	var averageSpeed float32

	x, y := pitLaneSpline.Dimensions()

	totalLength := float32(math.Sqrt(math.Pow(float64(x), 2) + math.Pow(float64(y), 2)))

	for _, point := range pitLaneSpline.ExtraPoints {
		averageSpeed += point.Speed
	}

	averageSpeed = averageSpeed / float32(len(pitLaneSpline.ExtraPoints))
	pitLaneTime := totalLength / averageSpeed

	return &PitLane{
		PitLaneSpline:      pitLaneSpline,
		TrackSpline:        trackSpline,
		PitLanePoints:      pitLanePoints,
		TrackPoints:        trackPoints,
		PitLaneCapable:     true,
		AveragePitLaneTime: time.Second * time.Duration(pitLaneTime*0.8),
	}, nil
}

func (p *PitLane) IsInPits(carUpdate udp.CarUpdate) bool {
	if !p.PitLaneCapable {
		return false
	}

	pitLanePoints, trackPoints := make([]acserver.Vector3F, len(p.PitLanePoints)), make([]acserver.Vector3F, len(p.TrackPoints))

	copy(pitLanePoints, p.PitLanePoints)
	copy(trackPoints, p.TrackPoints)

	position := carUpdate.Pos

	sort.Slice(pitLanePoints, func(i, j int) bool {
		return pitLanePoints[i].DistanceTo(position) < pitLanePoints[j].DistanceTo(position)
	})

	sort.Slice(trackPoints, func(i, j int) bool {
		return trackPoints[i].DistanceTo(position) < trackPoints[j].DistanceTo(position)
	})

	distanceToIdealLine := trackPoints[0].DistanceTo(position)
	distanceToPitLaneLine := pitLanePoints[0].DistanceTo(position)

	return distanceToIdealLine > distanceToPitLaneLine
}
