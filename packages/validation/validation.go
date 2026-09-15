// Package validation contains trust-boundary rules shared by Kinosail applications.
package validation

import (
	"errors"
	"net"
	"net/url"
	"path/filepath"
)

// ValidateRemoteAccess validates coupled remote-access settings.
func ValidateRemoteAccess(value func(string) string) error {
	if value == nil {
		return errors.New("remote access configuration is unavailable")
	}
	if _, _, err := net.SplitHostPort(value("remote.listen")); err != nil {
		return errors.New("remote.listen must be a valid listening address")
	}
	if gateway := value("remote.gateway"); gateway != "" && gateway != "false" && (gateway != "true" || value("remote.mode") != "https") {
		return errors.New("remote.gateway requires public HTTPS mode and a boolean value")
	}
	switch value("remote.mode") {
	case "off":
		return nil
	case "wireguard":
		return validateWireGuard(value)
	case "https":
		return validatePublicHTTPS(value)
	default:
		return errors.New("remote.mode must be off, wireguard, or https")
	}
}

func validateWireGuard(value func(string) string) error {
	if _, err := validateRemoteCredentials(value); err != nil {
		return err
	}
	directory := value("remote.wireguard_dir")
	if !filepath.IsAbs(directory) || filepath.Clean(directory) != directory {
		return errors.New("WireGuard directory must be an absolute clean path")
	}
	return nil
}

func validatePublicHTTPS(value func(string) string) error {
	domain, err := validateRemoteCredentials(value)
	if err != nil {
		return err
	}
	origin, err := url.Parse(value("auth.url"))
	if err != nil || origin.Scheme != "https" || origin.Host != domain+".duckdns.org" || origin.Path != "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		return errors.New("auth.url must be the configured DuckDNS HTTPS origin")
	}
	return nil
}

func validateRemoteCredentials(value func(string) string) (string, error) {
	domain := value("remote.duckdns_domain")
	if domain == "" || value("remote.duckdns_token") == "" {
		return "", errors.New("DuckDNS domain and token are required for remote access")
	}
	return domain, nil
}

// ValidRemoteToken accepts the bounded ASCII alphabet used by DuckDNS.
func ValidRemoteToken(token string) bool {
	return len(token) >= 32 && len(token) <= 128 && allRunes(token, remoteTokenCharacter)
}

// ValidSCIMToken accepts bounded visible ASCII without whitespace.
func ValidSCIMToken(token string) bool {
	return len(token) >= 32 && len(token) <= 256 && allRunes(token, visibleASCII)
}

// ValidateSupporter validates both configured Supporter endpoints.
func ValidateSupporter(activationRaw, supportRaw string) error {
	activation, err := url.Parse(activationRaw)
	if err != nil || !ValidSupporterURL(activation, true) || activation.RawQuery != "" {
		return errors.New("supporter.activation_url must be an absolute HTTPS URL without credentials or a fragment")
	}
	support, err := url.Parse(supportRaw)
	if err != nil || !ValidSupporterURL(support, false) {
		return errors.New("supporter.url must be an absolute HTTPS URL without credentials or a fragment")
	}
	return nil
}

// ValidSupporterURL accepts HTTPS or an explicitly allowed loopback HTTP endpoint.
func ValidSupporterURL(candidate *url.URL, allowLoopbackHTTP bool) bool {
	if candidate == nil || candidate.Host == "" || candidate.User != nil || candidate.Fragment != "" {
		return false
	}
	if candidate.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(candidate.Hostname())
	loopback := candidate.Hostname() == "localhost" || ip != nil && ip.IsLoopback()
	return allowLoopbackHTTP && candidate.Scheme == "http" && loopback
}

func allRunes(value string, allowed func(rune) bool) bool {
	for _, character := range value {
		if !allowed(character) {
			return false
		}
	}
	return true
}

func remoteTokenCharacter(character rune) bool {
	return lowercaseLabelCharacter(character) || character >= 'A' && character <= 'Z' || character == '_'
}

func lowercaseLabelCharacter(character rune) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '-'
}

func visibleASCII(character rune) bool { return character >= 0x21 && character <= 0x7e }
