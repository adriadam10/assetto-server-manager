package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/ai"
)

var aiFilePath string

func init() {
	flag.StringVar(&aiFilePath, "f", "pit_lane.ai", "ai file to parse")
	flag.Parse()
}

// yas marina
// anglesey
// f2f china acrl
// maple valley short - is the pitlane really half the track?
// trento bonde -- nowhere near track.

func main() {
	wd, _ := os.Getwd()

	var paths []string

	if err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Name() == "pit_lane.ai" {
			paths = append(paths, filepath.Dir(path))

			fmt.Println(filepath.Join(filepath.Dir(path)))
		}
		return nil
	}); err != nil {
		panic(err)
	}

	for _, path := range paths {
		path := path
		fmt.Println(path)

		fastLaneSpline, err := ai.ReadSpline(filepath.Join(path, "fast_lane.ai"))

		if err != nil {
			fmt.Println(err)
			continue
		}

		fullPitLane, err := ai.ReadSpline(filepath.Join(path, "pit_lane.ai"))

		if err != nil {
			fmt.Println(err)
			continue
		}

		drsZonesPath := filepath.Join(path, "..", "data", "drs_zones.ini")

		drsZones, err := acserver.LoadDRSZones(drsZonesPath)

		if err != nil {
			fmt.Println(err)
		}

		renderer := ai.NewTrackMapRenderer(fastLaneSpline, fullPitLane, drsZones)

		f, _ := os.Create(filepath.Join(wd, "maps", strings.Replace(filepath.ToSlash(path), "/", "_", -1)+"_map.png"))
		defer f.Close()

		_, err = renderer.Render(f)

		if err != nil {
			fmt.Println(err)
			continue
		}
	}
}
