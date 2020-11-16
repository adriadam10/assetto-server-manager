package acsm

import (
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/sirupsen/logrus"
)

type ContentType string

const (
	ContentTypeCar     = "Car"
	ContentTypeTrack   = "Track"
	ContentTypeWeather = "Weather"
)

type ContentFile struct {
	Name     string `json:"name"`
	FileType string `json:"type"`
	FilePath string `json:"filepath"`
	Data     string `json:"dataBase64"`
	Size     int    `json:"size"`
}

type UploadPayload struct {
	Files             []ContentFile `json:"Files"`
	GenerateTrackMaps bool          `json:"GenerateTrackMaps"`
}

var base64HeaderRegex = regexp.MustCompile("^(data:.+;base64,)")

type ContentUploadHandler struct {
	*BaseHandler

	carManager   *CarManager
	trackManager *TrackManager
}

func NewContentUploadHandler(baseHandler *BaseHandler, carManager *CarManager, trackManager *TrackManager) *ContentUploadHandler {
	return &ContentUploadHandler{
		BaseHandler:  baseHandler,
		carManager:   carManager,
		trackManager: trackManager,
	}
}

// Stores Files encoded into r.Body
func (cuh *ContentUploadHandler) upload(contentType ContentType) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var uploadPayload UploadPayload

		err := json.NewDecoder(r.Body).Decode(&uploadPayload)

		if err != nil {
			logrus.WithError(err).Errorf("could not decode %s json", contentType)
			return
		}

		err = cuh.handleUploadPayload(uploadPayload, contentType)

		if err != nil {
			logrus.WithError(err).Error("couldn't upload file")
			AddErrorFlash(w, r, string(contentType)+"(s) could not be added")
			return
		}

		AddFlash(w, r, string(contentType)+"(s) added successfully!")
	}
}

func (cuh *ContentUploadHandler) handleUploadPayload(payload UploadPayload, contentType ContentType) error {
	var contentPath string

	switch contentType {
	case ContentTypeTrack:
		contentPath = filepath.Join(ServerInstallPath, "content", "tracks")
	case ContentTypeCar:
		contentPath = filepath.Join(ServerInstallPath, "content", "cars")
	case ContentTypeWeather:
		contentPath = filepath.Join(ServerInstallPath, "content", "weather")
	}

	uploadedCars := make(map[string]bool)
	uploadedTracks := make(map[string]bool)

	var tags []string

	for _, file := range payload.Files {
		if file.Name == "tags" {
			tags = strings.Split(file.Data, ",")
			continue
		}

		var fileDecoded []byte

		if file.Size > 0 {
			// zero-size files will still be created, just with no content. (some data files exist but are empty)
			var err error
			fileDecoded, err = base64.StdEncoding.DecodeString(base64HeaderRegex.ReplaceAllString(file.Data, ""))

			if err != nil {
				logrus.WithError(err).Errorf("could not decode %s file data", contentType)
				return err
			}
		}

		// If user uploaded a "tracks" or "cars" folder containing multiple tracks
		parts := strings.Split(file.FilePath, "/")

		if parts[0] == "tracks" || parts[0] == "cars" || parts[0] == "weather" {
			parts = parts[1:]
			file.FilePath = ""

			for _, part := range parts {
				file.FilePath = filepath.Join(file.FilePath, part)
			}
		}

		path := filepath.Join(contentPath, file.FilePath)

		// Makes any directories in the path that don't exist (there can be multiple)
		err := os.MkdirAll(filepath.Dir(path), 0755)

		if err != nil {
			logrus.WithError(err).Errorf("could not create %s file directory", contentType)
			return err
		}

		if contentType == ContentTypeCar {
			uploadedCars[parts[0]] = true
		} else if contentType == ContentTypeTrack {
			uploadedTracks[parts[0]] = true
		}

		err = ioutil.WriteFile(path, fileDecoded, 0644)

		if err != nil {
			logrus.WithError(err).Error("could not write file")
			return err
		}
	}

	if contentType == ContentTypeCar {
		// index the cars that have been uploaded.
		for car := range uploadedCars {
			car, err := cuh.carManager.LoadCar(car, nil)

			if err != nil {
				return err
			}

			for _, tag := range tags {
				car.Details.AddTag(strings.TrimSpace(tag))
			}

			err = cuh.carManager.IndexCar(car)

			if err != nil {
				return err
			}

			err = cuh.carManager.SaveCarDetails(car.Name, car)

			if err != nil {
				return err
			}
		}
	}

	if contentType == ContentTypeTrack && payload.GenerateTrackMaps {
		for track := range uploadedTracks {
			t, err := cuh.trackManager.GetTrackFromName(track)

			if err != nil {
				return err
			}

			for _, layout := range t.Layouts {
				layoutForWarn := layout

				if layout == defaultLayoutName {
					layout = ""
					layoutForWarn = "default"
				}

				if err := cuh.trackManager.BuildTrackMap(track, layout); err != nil {
					logrus.WithError(err).Warnf("Track layout (%s, %s) was uploaded but AI spline files are missing or out of date, some advanced SM features will be unavailable for this layout!", track, layoutForWarn)
				}
			}
		}
	}

	return nil
}
