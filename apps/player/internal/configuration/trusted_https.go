package configuration

import "github.com/MikeO7/kinosail/packages/configurationcore"

func validateTrustedHTTPS(configured Snapshot) error {
	return configurationcore.ValidateTrustedHTTPS(configured)
}

func TrustedOrigin(raw, listen string) (string, error) {
	return configurationcore.TrustedOrigin(raw, listen)
}
