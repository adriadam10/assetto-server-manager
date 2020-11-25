package license

import (
	"bytes"
	"encoding/gob"
	"time"

	"github.com/google/uuid"
	"github.com/hyperboloide/lk"
)

func init() {
	gob.Register(License{})
}

type License struct {
	ID          uuid.UUID
	Email       string
	Provisioned time.Time
	Expires     time.Time
}

func GenerateLicense(privateKeyB32Encoded string, email string, expiry time.Time) (license *License, encoded string, err error) {
	license = &License{
		ID:          uuid.New(),
		Email:       email,
		Provisioned: time.Now(),
		Expires:     expiry,
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

const (
	Filename = "ACSM.License"

	licensePreamble  = "-----BEGIN LICENSE-----\n"
	licensePostamble = "\n-----END LICENSE-----"
)

func formatLicense(encoded string) string {
	out := licensePreamble

	for i := 0; i < len(encoded); i++ {
		out += string(encoded[i])

		if i > 0 && i%50 == 0 {
			out += "\n"
		}
	}

	out += licensePostamble

	return out
}
