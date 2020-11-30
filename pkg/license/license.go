package license

import (
	"bytes"
	"encoding/gob"
	"errors"
	"io/ioutil"
	"strings"
	"time"

	"github.com/hyperboloide/lk"
	"github.com/sirupsen/logrus"
)

const publicKeyBase32Encoded = "ARDCMQ757URYX7O35WCVWLSHLNWJ67MAK5FGEBWI2XGH5J5EWCBNWY7QEAKGX4O2FVVTFCM4ZEENQVCTYWH4IKY4TQGYLSMZTXZIJGML3OHUILQDM7OJNMAOCA6HIS5TXI76I5VJWCFQNKBFO27EROVBOBZQ===="

func GetLicense() *License {
	return loadedLicense
}

var (
	loadedLicense *License

	ErrLicenseInvalid = errors.New("license: invalid license specified")
	ErrLicenseExpired = errors.New("license: license has expired")
)

func LoadAndValidateLicense(filename string) error {
	publicKey, err := lk.PublicKeyFromB32String(publicKeyBase32Encoded)

	if err != nil {
		return err
	}

	b, err := ioutil.ReadFile(filename)

	if err != nil {
		return err
	}

	license, err := lk.LicenseFromB32String(parseLicense(string(b)))

	if err != nil {
		return err
	}

	if ok, err := license.Verify(publicKey); err != nil {
		return err
	} else if !ok {
		return ErrLicenseInvalid
	}

	if err := gob.NewDecoder(bytes.NewReader(license.Data)).Decode(&loadedLicense); err != nil {
		return err
	}

	if !loadedLicense.Expires.IsZero() && time.Now().After(loadedLicense.Expires) {
		return ErrLicenseExpired
	}

	logrus.Infof("This copy of ACSM is licensed to: %s", loadedLicense.Email)
	logrus.Infof("License created at: %s", loadedLicense.Provisioned.Format(time.ANSIC))

	if loadedLicense.Expires.IsZero() {
		logrus.Infof("License expires: never")
	} else {
		logrus.Infof("License expires: %s", loadedLicense.Expires.Format(time.ANSIC))
	}

	return nil
}

func parseLicense(l string) string {
	// windows line endings
	l = strings.Replace(l, "\r\n", "\n", -1)

	l = strings.TrimSpace(l)
	l = strings.TrimPrefix(l, licensePrefix)
	l = strings.TrimSuffix(l, licenseSuffix)

	l = strings.Replace(l, "\n", "", -1)

	return l
}
