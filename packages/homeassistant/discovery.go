package homeassistant

import (
	"errors"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/libp2p/zeroconf/v2"
)

type (
	discovery     interface{ Shutdown() }
	advertiseFunc func(string, int, []string) (discovery, error)
)

func advertiseHomeAssistant(name string, port int, text []string) (discovery, error) {
	if err := validateAdvertisement(name, port, text); err != nil {
		return nil, err
	}
	return zeroconf.Register(name, "_kinosail._tcp", "local.", port, text, nil)
}

func (integration *Integration[P]) startDiscoveryLocked() { //nolint:cyclop // Discovery setup keeps its lifecycle checks together.
	if integration.discovery != nil || integration.config.Lifecycle == nil {
		return
	}
	parsed, err := url.Parse(integration.config.AuthURL)
	if err != nil || parsed.Hostname() == "" {
		return
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		if parsed.Scheme == "https" {
			port = 443
		} else {
			port = 80
		}
	}
	server := integration.config.Server()
	text := []string{"id=" + server.ID, "version=1", "tls=" + strconv.FormatBool(parsed.Scheme == "https")}
	trusted := integration.config.TrustedHTTPS()
	text = append(text, "verify_ssl="+strconv.FormatBool(trusted.Configured && strings.EqualFold(parsed.Hostname(), trusted.Hostname)))
	if len(integration.config.AuthURL) <= 240 && !strings.EqualFold(parsed.Hostname(), "localhost") && !net.ParseIP(parsed.Hostname()).IsLoopback() {
		text = append(text, "url="+integration.config.AuthURL)
	}
	service, err := integration.advertise(server.Name, port, text)
	if err != nil {
		slog.Warn("Home Assistant discovery unavailable", "error", err)
		return
	}
	integration.discovery = service
	go func() {
		<-integration.config.Lifecycle.Done()
		integration.mu.Lock()
		integration.stopDiscoveryLocked()
		integration.mu.Unlock()
	}()
}

func (integration *Integration[P]) stopDiscoveryLocked() {
	if integration.discovery != nil {
		integration.discovery.Shutdown()
		integration.discovery = nil
	}
}

// Validate the complete DNS-SD record before opening multicast sockets.
func validateAdvertisement(name string, port int, text []string) error {
	if !validDiscoveryText(name, 63) || strings.TrimSpace(name) != name || port < 1 || port > 65535 || len(text) < 1 || len(text) > 5 {
		return errors.New("invalid discovery advertisement")
	}
	return validateDiscoveryTXT(text)
}

func validateDiscoveryTXT(text []string) error {
	seen := make(map[string]bool, len(text))
	for _, entry := range text {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !validDiscoveryText(entry, 255) || value == "" || seen[key] || !validDiscoveryValue(key, value) {
			return errors.New("invalid discovery TXT record")
		}
		seen[key] = true
	}
	if !seen["id"] {
		return errors.New("discovery id is required")
	}
	return nil
}

func validDiscoveryValue(key, value string) bool {
	switch key {
	case "id":
		return true
	case "version":
		return value == "1"
	case "tls", "verify_ssl":
		return value == "true" || value == "false"
	case "url":
		return validDiscoveryURL(value)
	default:
		return false
	}
}

func validDiscoveryText(value string, limit int) bool {
	return value != "" && len(value) <= limit && utf8.ValidString(value) && !strings.ContainsFunc(value, unicode.IsControl)
}

func validDiscoveryURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil
}
