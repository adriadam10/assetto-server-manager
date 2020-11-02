package acserver

import "github.com/cj123/ini"

type DRSZone struct {
	Detection float32
	Start     float32
	End       float32
}

func LoadDRSZones(drsZonesPath string) (map[string]DRSZone, error) {
	drsFile, err := ini.Load(drsZonesPath)

	if err != nil {
		return nil, err
	}

	drsZones := make(map[string]DRSZone)

	for _, section := range drsFile.Sections() {
		if section.Name() == "DEFAULT" {
			continue
		}

		detection, err := section.Key("DETECTION").Float64()

		if err != nil {
			return nil, err
		}

		start, err := section.Key("START").Float64()

		if err != nil {
			return nil, err
		}

		end, err := section.Key("END").Float64()

		if err != nil {
			return nil, err
		}

		drsZones[section.Name()] = DRSZone{
			Detection: float32(detection),
			Start:     float32(start),
			End:       float32(end),
		}
	}

	return drsZones, nil
}
