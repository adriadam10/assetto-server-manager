package main

import (
	"encoding/json"
	"flag"
	"os"

	"justapengu.in/acsm/pkg/ai"
)

var aiFilePath string

func init() {
	flag.StringVar(&aiFilePath, "f", "pit_lane.ai", "ai file to parse")
	flag.Parse()
}

func main() {
	spline, err := ai.ReadSpline(aiFilePath)

	if err != nil {
		panic(err)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(spline)
}
