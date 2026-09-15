package configuration

import "errors"

func validateJellyfinCompatibility(configured Snapshot) error {
	if configured.Bool("integrations.jellyfin.enabled") && configured.String("tls.duckdns") == "" {
		return errors.New("trusted HTTPS is required when Jellyfin compatibility is enabled")
	}
	return nil
}
