package plugins

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"justapengu.in/acsm/internal/acserver"
	"justapengu.in/acsm/pkg/ai"
)

type PitLanePlugin struct {
	serverInstallPath string
	pitLaneSpline     *ai.Spline
	trackSpline       *ai.Spline

	pitLanePoints, trackPoints []acserver.Vector3F

	pitLaneCapable bool

	server acserver.ServerPlugin
	logger acserver.Logger
}

func NewPitLanePlugin(serverInstallPath string) *PitLanePlugin {
	return &PitLanePlugin{serverInstallPath: serverInstallPath}
}

func (p *PitLanePlugin) Init(server acserver.ServerPlugin, logger acserver.Logger) error {
	p.server = server
	p.logger = logger

	p.logger.Infof("Pit lane plugin initialised")

	return nil
}

func (p *PitLanePlugin) OnVersion(version uint16) error {
	return nil
}

func (p *PitLanePlugin) OnNewSession(newSession acserver.SessionInfo) (err error) {
	defer func() {
		if err != nil {
			p.logger.WithError(err).Error("Track: %s (%s) does not have enough information for pitlane insights")
			p.pitLaneCapable = false
			err = nil
		} else {
			p.pitLaneCapable = true
		}
	}()

	p.trackSpline, err = ai.ReadSpline(filepath.Join(p.serverInstallPath, "content", "tracks", newSession.Track, newSession.TrackConfig, "ai", "fast_lane.ai"))

	if err != nil {
		return err
	}

	p.pitLaneSpline, err = ai.ReadPitLaneSpline(filepath.Join(p.serverInstallPath, "content", "tracks", newSession.Track, newSession.TrackConfig, "ai"))

	if err != nil {
		return err
	}

	for _, point := range p.pitLaneSpline.Points {
		p.pitLanePoints = append(p.pitLanePoints, point.Position)
	}

	for _, point := range p.trackSpline.Points {
		p.trackPoints = append(p.trackPoints, point.Position)
	}

	if len(p.pitLanePoints) == 0 {
		return errors.New("plugins: no pitlane points found")
	}

	return nil
}

func (p *PitLanePlugin) OnWeatherChange(weather acserver.CurrentWeather) error {

	return nil
}

func (p *PitLanePlugin) OnEndSession(sessionFile string) error {

	return nil
}

func (p *PitLanePlugin) OnNewConnection(car acserver.Car) error {

	return nil
}

func (p *PitLanePlugin) OnClientLoaded(car acserver.Car) error {

	return nil
}

func (p *PitLanePlugin) OnSectorCompleted(split acserver.Split) error {

	return nil
}

func (p *PitLanePlugin) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {

	return nil
}

func (p *PitLanePlugin) OnCarUpdate(carUpdate acserver.Car) error {
	if !p.pitLaneCapable {
		return nil
	}

	pitLanePoints, trackPoints := make([]acserver.Vector3F, len(p.pitLanePoints)), make([]acserver.Vector3F, len(p.trackPoints))

	copy(pitLanePoints, p.pitLanePoints)
	copy(trackPoints, p.trackPoints)

	position := carUpdate.PluginStatus.Position

	sort.Slice(pitLanePoints, func(i, j int) bool {
		return pitLanePoints[i].DistanceTo(position) < pitLanePoints[j].DistanceTo(position)
	})

	sort.Slice(trackPoints, func(i, j int) bool {
		return trackPoints[i].DistanceTo(position) < trackPoints[j].DistanceTo(position)
	})

	distanceToIdealLine := trackPoints[0].DistanceTo(position)
	distanceToPitLaneLine := pitLanePoints[0].DistanceTo(position)

	if distanceToIdealLine < distanceToPitLaneLine {
		fmt.Println("ON TRACK")
	} else {
		fmt.Println("IN PITLANE")
	}

	return nil
}

func (p *PitLanePlugin) OnTyreChange(car acserver.Car, tyres string) error {

	return nil
}

func (p *PitLanePlugin) OnClientEvent(event acserver.ClientEvent) error {

	return nil
}

func (p *PitLanePlugin) OnCollisionWithCar(event acserver.ClientEvent) error {

	return nil
}

func (p *PitLanePlugin) OnCollisionWithEnv(event acserver.ClientEvent) error {

	return nil
}

func (p *PitLanePlugin) OnChat(chat acserver.Chat) error {

	return nil
}

func (p *PitLanePlugin) OnConnectionClosed(car acserver.Car) error {

	return nil
}
