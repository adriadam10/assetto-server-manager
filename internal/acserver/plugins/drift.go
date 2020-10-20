package plugins

import (
	"fmt"
	"math"
	"time"

	"github.com/sirupsen/logrus"

	"justapengu.in/acsm/internal/acserver"
)

type DriftPlugin struct {
	server      acserver.ServerPlugin
	logger      acserver.Logger

	drifters []*drifter
}

type drifter struct {
	carID acserver.CarID
	name  string

	currentDriftStart time.Time
	currentDriftScore float64
	currentLapScore   float64

	lastLapScore float64
	bestLapScore float64
}

func NewDriftPlugin() *DriftPlugin {
	return &DriftPlugin{}
}

func (d *DriftPlugin) Init(server acserver.ServerPlugin, logger acserver.Logger) error {
	d.server = server
	d.logger = logger

	return nil
}

func (d *DriftPlugin) OnNewConnection(car acserver.CarInfo) error {
	_ = d.OnConnectionClosed(car)

	d.drifters = append(d.drifters, &drifter{
		carID: car.CarID,
		name:  car.Driver.Name,
	})

	return nil
}

func (d *DriftPlugin) OnConnectionClosed(car acserver.CarInfo) error {
	for i, drifter := range d.drifters {
		if drifter.carID == car.CarID {
			copy(d.drifters[i:], d.drifters[i+1:])
			d.drifters[len(d.drifters)-1] = nil
			d.drifters = d.drifters[:len(d.drifters)-1]

			break
		}
	}

	return nil
}

func (d *DriftPlugin) OnCarUpdate(car acserver.CarInfo) error {
	var drifter *drifter

	for _, existingDrifter := range d.drifters {
		if existingDrifter.carID == car.CarID {
			drifter = existingDrifter
			break
		}
	}

	if drifter == nil {
		logrus.Warnf("Car %d sent a drift update, but isn't in the drift car list!", car.CarID)

		return nil
	}

	velocityMagnitude := car.PluginStatus.Velocity.Magnitude()

	if velocityMagnitude > 3 {
		// velocity vector rotation relative to track x axis
		velocityRotation := getVectorAngle(float64(car.PluginStatus.Velocity.X), float64(car.PluginStatus.Velocity.Z))
		// vehicle rotation relative to track z axis (why is it not x, we will never know)
		rotation := radsToDegrees(float64(car.PluginStatus.Rotation.X))

		diff := math.Abs(rotation - velocityRotation)

		if 360-diff < diff {
			diff = 360 - diff
		}

		driftAngle := math.Abs(90 - diff) // due to velocity and rotation being relative to different axes

		if driftAngle > 5 {
			if drifter.currentDriftStart.IsZero() {
				drifter.currentDriftStart = time.Now()
			}

			drifter.currentDriftScore += driftAngle * time.Since(drifter.currentDriftStart).Seconds() * (0.1 * velocityMagnitude)
		} else {
			if drifter.currentDriftScore > 0 {
				drifter.currentLapScore += drifter.currentDriftScore

				err := d.server.SendChat(fmt.Sprintf("%.0f points for drift!", drifter.currentDriftScore), acserver.ServerCarID, car.CarID, true)

				if err != nil {
					d.logger.WithError(err).Error("Send chat returned an error")
				}
			}

			drifter.currentDriftStart = time.Time{}
			drifter.currentDriftScore = 0
		}
	} else {
		// spin out, reduce points
		if drifter.currentDriftScore > 0 {
			drifter.currentLapScore += drifter.currentDriftScore / 2

			err := d.server.SendChat(fmt.Sprintf("Spinout! %.0f points for drift!", drifter.currentDriftScore/2), acserver.ServerCarID, car.CarID, true)

			if err != nil {
				d.logger.WithError(err).Error("Send chat returned an error")
			}
		}

		drifter.currentDriftStart = time.Time{}
		drifter.currentDriftScore = 0
	}

	return nil
}

func getVectorAngle(dx, dz float64) float64 {
	inRads := math.Atan2(dz, dx)

	return radsToDegrees(inRads)
}

func radsToDegrees(inRads float64) float64 {
	if inRads < 0 {
		inRads = math.Abs(inRads)
	} else {
		inRads = 2*math.Pi - inRads
	}

	return inRads * (180 / math.Pi)
}

func (d *DriftPlugin) OnNewSession(newSession acserver.SessionInfo) error {
	for _, drifter := range d.drifters {
		drifter.currentDriftScore = 0
		drifter.currentDriftStart = time.Time{}
		drifter.currentLapScore = 0
		drifter.bestLapScore = 0
		drifter.lastLapScore = 0
	}

	return nil
}

func (d *DriftPlugin) OnEndSession(sessionFile string) error {
	return nil
}

func (d *DriftPlugin) OnVersion(version uint16) error {
	return nil
}

func (d *DriftPlugin) OnChat(chat acserver.Chat) error {
	return nil
}

func (d *DriftPlugin) OnClientLoaded(car acserver.CarInfo) error {
	return nil
}

func (d *DriftPlugin) OnLapCompleted(carID acserver.CarID, lap acserver.Lap) error {
	var drifter *drifter

	for _, existingDrifter := range d.drifters {
		if existingDrifter.carID == carID {
			drifter = existingDrifter
			break
		}
	}

	if drifter == nil {
		logrus.Warnf("Car %d completed a lap, but isn't in the drift car list!", carID)

		return nil
	}

	if drifter.currentLapScore > 0 {
		drifter.lastLapScore = drifter.currentLapScore

		best := "new"

		if drifter.currentLapScore > drifter.bestLapScore {
			drifter.bestLapScore = drifter.currentLapScore

			best = "personal best"
		}

		d.server.BroadcastChat(fmt.Sprintf("%s set a %s lap! %.0f drift points!", drifter.name, best, drifter.currentLapScore), acserver.ServerCarID, true)

		drifter.currentLapScore = 0
	}

	return nil
}

func (d *DriftPlugin) OnClientEvent(_ acserver.ClientEvent) error {
	return nil
}

// @TODO should collisions cost points? OR GIVE POINTS (tail taps)
func (d *DriftPlugin) OnCollisionWithCar(event acserver.ClientEvent) error {
	return nil
}

func (d *DriftPlugin) OnCollisionWithEnv(event acserver.ClientEvent) error {
	return nil
}

func (d *DriftPlugin) OnSectorCompleted(car acserver.CarInfo, split acserver.Split) error {
	return nil
}

func (d *DriftPlugin) OnWeatherChange(_ acserver.CurrentWeather) error {
	return nil
}

func (d *DriftPlugin) OnTyreChange(car acserver.CarInfo, tyres string) error {
	return nil
}

func (d *DriftPlugin) SortLeaderboard(_ acserver.SessionType, _ []*acserver.LeaderboardLine) {

}
