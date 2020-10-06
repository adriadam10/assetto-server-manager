package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

	var wg sync.WaitGroup

	for _, path := range paths {
		path := path
		wg.Add(1)

		go func() {
			pitlaneSpline, err := ai.ReadPitLaneSpline(path)

			if err != nil {
				fmt.Println(err)
				return
			}

			fastlaneSpline, err := ai.ReadSpline(filepath.Join(path, "fast_lane.ai"))

			if err != nil {
				fmt.Println(err)
				return
			}

			x, y := fastlaneSpline.Dimensions()

			padding := 20
			img := image.NewRGBA(image.Rectangle{Min: image.Pt(0, 0), Max: image.Pt(int(x)+(padding*2), int(y)+(padding*2))})
			radius := 1

			minX, minY := fastlaneSpline.Min()

			for i, point := range fastlaneSpline.Points {
				extra := fastlaneSpline.ExtraPoints[i]

				left := point.Position.Sub(extra.Normal.Mul(extra.SideLeft))
				right := point.Position.Add(extra.Normal.Mul(extra.SideRight))

				draw.Draw(img, img.Bounds(), &circle{image.Pt(padding+int(point.Position.X-minX), padding+int(point.Position.Z-minY)), radius, color.RGBA{R: 0, G: 125, B: 0, A: 0xff}}, image.Pt(0, 0), draw.Over)

				draw.Draw(img, img.Bounds(), &circle{image.Pt(padding+int(left.X-minX), padding+int(left.Z-minY)), radius, color.RGBA{R: 150, G: 0, B: 0, A: 0xff}}, image.Pt(0, 0), draw.Over)
				draw.Draw(img, img.Bounds(), &circle{image.Pt(padding+int(right.X-minX), padding+int(right.Z-minY)), radius, color.RGBA{R: 0, G: 0, B: 150, A: 0xff}}, image.Pt(0, 0), draw.Over)
			}

			for _, point := range pitlaneSpline.Points {
				draw.Draw(img, img.Bounds(), &circle{image.Pt(padding+int(point.Position.X-minX), padding+int(point.Position.Z-minY)), radius, color.RGBA{R: 255, G: 125, B: 0, A: 0xff}}, image.Pt(0, 0), draw.Over)
			}

			f, _ := os.Create(filepath.Join(wd, "maps", strings.Replace(filepath.ToSlash(path), "/", "_", -1)+"_map.png"))
			defer f.Close()

			err = png.Encode(f, img)

			if err != nil {
				panic(err)
			}

			wg.Done()
		}()
	}

	wg.Wait()
}

type circle struct {
	p     image.Point
	r     int
	color color.RGBA
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
