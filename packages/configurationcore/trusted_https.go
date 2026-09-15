package configurationcore

import (
	"errors"
	"net"
	"net/url"
	"strconv"

	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

// Snapshot exposes the resolved values needed by trusted HTTPS policy.
type Snapshot interface {
	String(string) string
	Bool(string) bool
}

// ValidateTrustedHTTPS validates trusted HTTPS against the other network settings.
func ValidateTrustedHTTPS(configured Snapshot) error { //nolint:cyclop // The score of 11 remains below the repository ceiling of 22 for one cross-field validator.
	trusted, err := trustedhttps.Parse(configured.String("tls.duckdns"))
	if err != nil || trusted == (trustedhttps.Config{}) {
		return err
	}
	if !configured.Bool("tls.enabled") {
		return errors.New("tls.enabled is required for trusted HTTPS")
	}
	if configured.String("remote.mode") == "https" {
		return errors.New("LAN trusted HTTPS and public HTTPS remote access cannot both be enabled")
	}
	if trusted.ProviderName() == trustedhttps.ProviderDuckDNS && configured.String("remote.mode") != "off" && configured.String("remote.duckdns_domain") == trusted.Domain {
		return errors.New("LAN trusted HTTPS and remote access must use different DuckDNS subdomains")
	}
	expected, err := trustedOrigin(trusted, configured.String("listen"))
	if err != nil {
		return err
	}
	if raw := configured.String("auth.url"); raw != "" && raw != expected {
		return errors.New("auth.url must match the trusted HTTPS origin")
	}
	return nil
}

// TrustedOrigin returns the canonical browser and passkey origin.
func TrustedOrigin(raw, listen string) (string, error) {
	config, err := trustedhttps.Parse(raw)
	if err != nil || config == (trustedhttps.Config{}) {
		return "", err
	}
	return trustedOrigin(config, listen)
}

func trustedOrigin(config trustedhttps.Config, listen string) (string, error) {
	_, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "", errors.New("listen must be a valid listening address")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", errors.New("listen must use a numeric port from 1 to 65535")
	}
	host := config.Hostname()
	if port != "443" {
		host = net.JoinHostPort(host, port)
	}
	return (&url.URL{Scheme: "https", Host: host}).String(), nil
}
