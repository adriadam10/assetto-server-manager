// +build !licensed

package license

func IsLicensed() bool {
	return false
}

func GetLicense() *License {
	return nil
}

func LoadAndValidateLicense(licenseFile string) error {
	return nil
}
