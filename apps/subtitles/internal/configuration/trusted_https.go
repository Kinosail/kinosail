package configuration

import "github.com/MikeO7/kinosail/packages/configurationcore"

func validateTrustedHTTPS(configured Snapshot) error {
	return configurationcore.ValidateTrustedHTTPS(configured)
}

// TrustedOrigin returns the canonical browser and passkey origin for a stored LAN certificate setting.
func TrustedOrigin(raw, listen string) (string, error) {
	return configurationcore.TrustedOrigin(raw, listen)
}
