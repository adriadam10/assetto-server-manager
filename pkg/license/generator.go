package license

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/google/uuid"
	"github.com/hyperboloide/lk"
)

//go:generate stringer -type=ManagerType -linecomment

func init() {
	gob.Register(License{})
}

type License struct {
	ID          uuid.UUID
	Email       string
	Provisioned time.Time
	Expires     time.Time
	ManagerType ManagerType
}

func GenerateLicense(privateKeyB32Encoded string, email string, expiry time.Time, managerType ManagerType) (license *License, encoded string, err error) {
	license = &License{
		ID:          uuid.New(),
		Email:       email,
		Provisioned: time.Now(),
		Expires:     expiry,
		ManagerType: managerType,
	}

	privateKey, err := lk.PrivateKeyFromB32String(privateKeyB32Encoded)

	if err != nil {
		return nil, "", err
	}

	buf := new(bytes.Buffer)

	if err := gob.NewEncoder(buf).Encode(license); err != nil {
		return nil, "", err
	}

	key, err := lk.NewLicense(privateKey, buf.Bytes())

	if err != nil {
		return nil, "", err
	}

	encoded, err = key.ToB32String()

	return license, formatLicense(encoded), err
}

type ManagerType int

const (
	Filename = "ACSM.License"

	ManagerTypeACSM   ManagerType = iota // Assetto Corsa Server Manager
	ManagerTypeAMS2                      // Automobilista 2 Server Manager
	ManagerTypePCars2                    // Project Cars 2 Server Manager
	ManagerTypeACC                       // Assetto Corsa Competizione Server Manager
	ManagerTypeAny                       // Any Server Manager

	licensePrefix = "-----BEGIN LICENSE-----\n"
	licenseSuffix = "\n-----END LICENSE-----"
)

func formatLicense(encoded string) string {
	out := licensePrefix

	for i := 1; i <= len(encoded); i++ {
		out += string(encoded[i-1])

		if i%50 == 0 {
			out += "\n"
		}
	}

	out += licenseSuffix

	return out
}
