package acserver

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ChecksumFile struct {
	Filename string
	MD5      []byte
}

type CustomChecksumFile struct {
	Name     string `json:"name" yaml:"name"`
	Filename string `json:"file_path" yaml:"file_path"`
	MD5      string `json:"md5" yaml:"md5"`
}

type ChecksumManager struct {
	baseDirectory string
	state         *ServerState

	logger             Logger
	customChecksums    []CustomChecksumFile
	checkSummableFiles []ChecksumFile
}

func NewChecksumManager(baseDirectory string, state *ServerState, logger Logger, customChecksums []CustomChecksumFile) (*ChecksumManager, error) {
	cm := &ChecksumManager{
		baseDirectory:   baseDirectory,
		state:           state,
		logger:          logger,
		customChecksums: customChecksums,
	}

	return cm, cm.init()
}

var systemDataSurfacesPath = filepath.Join("system", "data", "surfaces.ini")

var checkSummableCarFiles = []string{
	"aero.ini",
	"ai.ini",
	"ambient_shadows.ini",
	"blurred_objects.ini",
	"brakes.ini",
	"cameras.ini",
	"car.ini",
	"colliders.ini",
	"damage.ini",
	"dash_cam.ini",
	"digital_instruments.ini",
	"driver3d.ini",
	"drivetrain.ini",
	"drs.ini",
}

func (cm *ChecksumManager) init() error {
	var trackSurfacesPath string
	var trackModelsPath string

	if cm.state.raceConfig.TrackLayout == "" {
		trackSurfacesPath = filepath.Join(cm.baseDirectory, "content", "tracks", cm.state.raceConfig.Track, "data", "surfaces.ini")
		trackModelsPath = filepath.Join(cm.baseDirectory, "content", "tracks", cm.state.raceConfig.Track, "models.ini")
	} else {
		trackSurfacesPath = filepath.Join(cm.baseDirectory, "content", "tracks", cm.state.raceConfig.Track, cm.state.raceConfig.TrackLayout, "data", "surfaces.ini")
		trackModelsPath = filepath.Join(cm.baseDirectory, "content", "tracks", cm.state.raceConfig.Track, "models_"+cm.state.raceConfig.TrackLayout+".ini")
	}

	filesToChecksum := []string{
		filepath.Join(cm.baseDirectory, systemDataSurfacesPath),
		trackSurfacesPath,
		trackModelsPath,
	}

	for _, car := range cm.state.raceConfig.Cars {
		acdFilepath := filepath.Join(cm.baseDirectory, "content", "cars", car, "data.acd")

		if _, err := os.Stat(acdFilepath); os.IsNotExist(err) {
			// this car is likely using a data folder rather than an acd file. checksum all files within the data path
			dataPath := filepath.Join(cm.baseDirectory, "content", "cars", car, "data")

			files, err := ioutil.ReadDir(dataPath)

			if err == nil {
				for _, file := range files {
					for _, checkSummableFile := range checkSummableCarFiles {
						if file.Name() == checkSummableFile {
							filesToChecksum = append(filesToChecksum, filepath.Join(dataPath, file.Name()))
						}
					}
				}
			} else if os.IsNotExist(err) {
				cm.logger.Warnf("Could not find data.acd or data folder for car: %s. Continuing without checksums", car)
				continue
			}
		} else if err != nil {
			return err
		} else {
			filesToChecksum = append(filesToChecksum, acdFilepath)
		}
	}

	cm.logger.Debugf("Running checksum for %d files", len(filesToChecksum)+len(cm.customChecksums))

	for _, file := range filesToChecksum {
		checksum, err := md5File(file)

		if os.IsNotExist(err) {
			cm.logger.Warnf("Could not find checksum file: %s", file)
			continue
		} else if err != nil {
			return err
		}

		relativePath, err := filepath.Rel(cm.baseDirectory, file)

		if err != nil {
			return err
		}

		relativePath = filepath.ToSlash(relativePath)

		cm.logger.Debugf("Checksum added: md5(%s)=%s", relativePath, hex.EncodeToString(checksum))

		cm.checkSummableFiles = append(cm.checkSummableFiles, ChecksumFile{Filename: relativePath, MD5: checksum})
	}

	for _, customChecksum := range cm.customChecksums {
		if !sanitiseChecksumPath(customChecksum.Filename) {
			continue
		}

		checksum, err := hex.DecodeString(customChecksum.MD5)

		if err != nil {
			cm.logger.WithError(err).Errorf("Couldn't decode checksum: %s", customChecksum.MD5)
		} else {
			cm.logger.Debugf("Checksum added from config: md5(%s)=%s", customChecksum.Filename, customChecksum.MD5)

			cm.checkSummableFiles = append(cm.checkSummableFiles, ChecksumFile{customChecksum.Filename, checksum})
		}
	}

	return nil
}

func (cm *ChecksumManager) GetFiles() []ChecksumFile {
	return cm.checkSummableFiles
}

func md5File(filepath string) ([]byte, error) {
	f, err := os.Open(filepath)

	if err != nil {
		return nil, err
	}

	defer f.Close()

	h := md5.New()

	_, err = io.Copy(h, f)

	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

var absPathRegex = regexp.MustCompile(`[A-Z]:`)

func sanitiseChecksumPath(path string) bool {
	cleanPath := filepath.Clean(path)

	if strings.HasPrefix(cleanPath, "..") {
		return false
	}

	if strings.HasPrefix(cleanPath, "\\") {
		return false
	}

	if filepath.IsAbs(cleanPath) {
		return false
	}

	if absPathRegex.MatchString(cleanPath) {
		return false
	}

	return true
}
