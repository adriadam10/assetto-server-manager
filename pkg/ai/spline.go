// package ai provides utilities for dealing with Assetto Corsa's AI files.
// it is ported to Go from the following repository: https://github.com/gro-ove/actools
package ai

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"justapengu.in/acsm/internal/acserver"
)

const version = int32(7)

type Spline struct {
	Version     int32
	NumPoints   int32
	LapTime     int32
	SampleCount int32

	Points      []Point
	ExtraPoints []Extra

	HasGrid bool
	Grid    Grid
}

func (s *Spline) Min() (float32, float32) {
	if len(s.Points) == 0 {
		return 0, 0
	}

	minX, minZ := s.Points[0].Position.X, s.Points[0].Position.Z

	for _, point := range s.Points {
		if point.Position.X < minX {
			minX = point.Position.X
		}

		if point.Position.Z < minZ {
			minZ = point.Position.Z
		}
	}

	return minX, minZ
}

func (s *Spline) Max() (float32, float32) {
	maxX, maxZ := s.Points[0].Position.X, s.Points[0].Position.Z

	for _, point := range s.Points {
		if point.Position.X > maxX {
			maxX = point.Position.X
		}

		if point.Position.Z > maxZ {
			maxZ = point.Position.Z
		}
	}

	return maxX, maxZ
}

func (s *Spline) Dimensions() (float32, float32) {
	minX, minZ := s.Min()
	maxX, maxZ := s.Max()

	return maxX - minX, maxZ - minZ
}

func (s Spline) Subtract(other *Spline, distance float64) *Spline {
	var newPoints []Point
	var extraPoints []Extra

	for i, point := range s.Points {
		exists := false

		for _, otherPoint := range other.Points {
			if point.Position.DistanceTo(otherPoint.Position) < distance {
				exists = true
				break
			}
		}

		if !exists {
			newPoints = append(newPoints, point)
			extraPoints = append(extraPoints, s.ExtraPoints[i])
		}
	}

	s.Points = newPoints
	s.ExtraPoints = extraPoints

	return &s
}

func (s Spline) FilterByMaxSpeed(maxSpeed float32) *Spline {
	var points []Point
	var extraPoints []Extra

	for i, point := range s.Points {
		extra := s.ExtraPoints[i]

		if extra.Speed < maxSpeed {
			points = append(points, point)
			extraPoints = append(extraPoints, extra)
		}
	}

	s.Points = points
	s.ExtraPoints = extraPoints

	return &s
}

func (s Spline) FindLargestContinuousSegment(maxDistance float64) *Spline {
	var splits [][]Point
	var extraSplits [][]Extra

	splitStartedAt := 0

	for i := 0; i < len(s.Points); i++ {
		if i > 0 {
			previousPoint := s.Points[i-1]

			if previousPoint.Position.DistanceTo(s.Points[i].Position) > maxDistance {
				// this is a split.
				splits = append(splits, s.Points[splitStartedAt:i])
				extraSplits = append(extraSplits, s.ExtraPoints[splitStartedAt:i])
				splitStartedAt = i + 1
			}
		}
	}

	if splitStartedAt > 0 || len(splits) == 0 {
		splits = append(splits, s.Points[splitStartedAt:])
		extraSplits = append(extraSplits, s.ExtraPoints[splitStartedAt:])
	}

	var biggestSplit []Point
	var biggestExtraSplit []Extra

	for i, split := range splits {
		if len(split) > len(biggestSplit) {
			biggestSplit = split

			if len(extraSplits) > 0 {
				biggestExtraSplit = extraSplits[i]
			}
		}
	}

	s.Points = biggestSplit
	s.ExtraPoints = biggestExtraSplit

	return &s
}

type Point struct {
	Position acserver.Vector3F
	Length   float32
	ID       int32
}

type Extra struct {
	Speed         float32
	Gas           float32
	Brake         float32
	ObsoleteLatG  float32
	Radius        float32
	SideLeft      float32
	SideRight     float32
	Camber        float32
	Direction     float32
	Normal        acserver.Vector3F
	Length        float32
	ForwardVector acserver.Vector3F
	Tag           float32
	Grade         float32
}

type Grid struct {
	MaxExtreme acserver.Vector3F
	MinExtreme acserver.Vector3F

	NeighboursConsideredNumber int32
	SamplingDensity            float32
	NumItems                   int32
	Items                      []GridItem
}

type GridItem struct {
	NumSubs int32

	Subs []GridItemSub
}

type GridItemSub struct {
	NumValues int32
	Values    []int32
}

func ReadSpline(aiFile string) (*Spline, error) {
	f, err := os.Open(aiFile)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	var spline Spline

	if err := binary.Read(f, binary.LittleEndian, &spline.Version); err != nil {
		return nil, err
	}

	if spline.Version != version {
		return nil, fmt.Errorf("ai: version: %d is not supported", spline.Version)
	}

	if err := binary.Read(f, binary.LittleEndian, &spline.NumPoints); err != nil {
		return nil, err
	}

	if err := binary.Read(f, binary.LittleEndian, &spline.LapTime); err != nil {
		return nil, err
	}

	if err := binary.Read(f, binary.LittleEndian, &spline.SampleCount); err != nil {
		return nil, err
	}

	for i := 0; i < int(spline.NumPoints); i++ {
		var point Point

		if err := binary.Read(f, binary.LittleEndian, &point); err != nil {
			return nil, err
		}

		spline.Points = append(spline.Points, point)
	}

	var extraPoints int32

	if err := binary.Read(f, binary.LittleEndian, &extraPoints); err != nil {
		return nil, err
	}

	for i := 0; i < int(extraPoints); i++ {
		var extra Extra

		if err := binary.Read(f, binary.LittleEndian, &extra); err != nil {
			return nil, err
		}

		spline.ExtraPoints = append(spline.ExtraPoints, extra)
	}

	var hasGrid int32

	if err := binary.Read(f, binary.LittleEndian, &hasGrid); err != nil {
		return nil, err
	}

	spline.HasGrid = hasGrid == 1

	if spline.HasGrid {
		if err := binary.Read(f, binary.LittleEndian, &spline.Grid.MaxExtreme); err != nil {
			return nil, err
		}

		if err := binary.Read(f, binary.LittleEndian, &spline.Grid.MinExtreme); err != nil {
			return nil, err
		}

		if err := binary.Read(f, binary.LittleEndian, &spline.Grid.NeighboursConsideredNumber); err != nil {
			return nil, err
		}

		if err := binary.Read(f, binary.LittleEndian, &spline.Grid.SamplingDensity); err != nil {
			return nil, err
		}

		if err := binary.Read(f, binary.LittleEndian, &spline.Grid.NumItems); err != nil {
			return nil, err
		}

		for i := 0; i < int(spline.Grid.NumItems); i++ {
			var item GridItem

			if err := binary.Read(f, binary.LittleEndian, &item.NumSubs); err != nil {
				return nil, err
			}

			for j := 0; j < int(item.NumSubs); j++ {
				var subItem GridItemSub

				if err := binary.Read(f, binary.LittleEndian, &subItem.NumValues); err != nil {
					return nil, err
				}

				for k := 0; k < int(subItem.NumValues); k++ {
					var value int32

					if err := binary.Read(f, binary.LittleEndian, &value); err != nil {
						return nil, err
					}

					subItem.Values = append(subItem.Values, value)
				}

				item.Subs = append(item.Subs, subItem)
			}

			spline.Grid.Items = append(spline.Grid.Items, item)
		}
	}

	return &spline, nil
}

func ReadPitLaneSpline(dir string, fastLaneSpline *Spline, maxSpeed float32, distance, maxDistance float64) (*Spline, error) {
	pitLaneFullSpline, err := ReadSpline(filepath.Join(dir, "pit_lane.ai"))

	if err != nil {
		return nil, err
	}

	return pitLaneFullSpline.Subtract(fastLaneSpline, distance).FilterByMaxSpeed(maxSpeed).FindLargestContinuousSegment(maxDistance), nil
}
