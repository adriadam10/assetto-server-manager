// package ai provides utilities for dealing with Assetto Corsa's AI files.
// it is ported to Go from the following repository: https://github.com/gro-ove/actools
package ai

import (
	"encoding/binary"
	"fmt"
	"os"
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

type Point struct {
	Position [3]float32
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
	Normal        [3]float32
	Length        float32
	ForwardVector [3]float32
	Tag           float32
	Grade         float32
}

type Grid struct {
	MaxExtreme [3]float32
	MinExtreme [3]float32

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
