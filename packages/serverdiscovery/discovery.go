// Package serverdiscovery advertises Player endpoints on the local network.
package serverdiscovery

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/libp2p/zeroconf/v2"
)

const ServiceType = "_kinosail-player._tcp"

type (
	advertisement interface{ Shutdown() }
	registerFunc  func(string, string, string, int, []string, []net.Interface) (advertisement, error)
)

// Start publishes only the server name and connection endpoint, until cancellation.
func Start(ctx context.Context, name, origin string) error {
	return start(ctx, name, origin, func(name, service, domain string, port int, txt []string, interfaces []net.Interface) (advertisement, error) {
		return zeroconf.Register(name, service, domain, port, txt, interfaces)
	})
}

func start(ctx context.Context, name, origin string, register registerFunc) error {
	port, txt, err := record(name, origin)
	if err != nil {
		return err
	}
	if ctx == nil || ctx.Done() == nil {
		return errors.New("discovery requires a lifecycle")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	service, err := register(name, ServiceType, "local.", port, txt, nil)
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); service.Shutdown() }()
	return nil
}

func record(name, origin string) (int, []string, error) {
	invalid := errors.New("invalid Player discovery advertisement")
	if !validAdvertisement(name, origin) {
		return 0, nil, invalid
	}
	parsed, err := url.Parse(origin)
	if err != nil || !validDiscoveryURL(parsed, origin) {
		return 0, nil, invalid
	}
	port, err := discoveryPort(parsed)
	if err != nil {
		return 0, nil, invalid
	}
	host := strings.ToLower(parsed.Hostname())
	ip := net.ParseIP(host)
	if !validDiscoveryHost(host, ip) {
		return 0, nil, invalid
	}
	txt := []string{"version=1", "scheme=" + parsed.Scheme}
	// Loopback addresses refer to the client itself: resolve the Bonjour host instead.
	if advertised, err := advertisedOrigin(parsed.Scheme, host, origin, ip); err != nil {
		return 0, nil, invalid
	} else if advertised != "" {
		txt = append(txt, "url="+advertised)
	}
	return port, txt, nil
}

// LoopbackAlias returns only the hostname advertised by zeroconf for a loopback origin.
// It lets the HTTP adapter accept this one local name without weakening its host allowlist.
func LoopbackAlias(origin string) string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return loopbackAlias(origin, hostname)
}

func loopbackAlias(origin, hostname string) string {
	_, txt, err := record("Player", origin)
	if err != nil || len(txt) != 2 {
		return ""
	}
	// Register appends its local domain to the operating-system hostname.
	hostname = strings.TrimSuffix(hostname, ".") + ".local"
	if !validHostname(hostname) {
		return ""
	}
	return strings.ToLower(hostname)
}

func validHostname(host string) bool {
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if !validHostnameLabel(label) {
			return false
		}
	}
	return true
}

func discoveryPort(parsed *url.URL) (int, error) {
	port := 80
	if parsed.Scheme == "https" {
		port = 443
	}
	if parsed.Port() != "" {
		var err error
		port, err = strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			return 0, errors.New("invalid Player discovery advertisement")
		}
	}
	if strings.HasSuffix(parsed.Host, ":") {
		return 0, errors.New("invalid Player discovery advertisement")
	}
	return port, nil
}

func validHostnameLabel(label string) bool {
	if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	return strings.IndexFunc(label, invalidHostnameCharacter) < 0
}

func validDiscoveryURL(parsed *url.URL, origin string) bool { //nolint:cyclop // Discovery accepts only a credential-free HTTP origin without query or fragment.
	return (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil && (parsed.Path == "" || parsed.Path == "/") && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" && !strings.ContainsAny(origin, "\\#")
}

func advertisedOrigin(scheme, host, origin string, ip net.IP) (string, error) {
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		if scheme == "http" && !strings.HasSuffix(host, ".local") && (ip == nil || !ip.IsPrivate() && !ip.IsLinkLocalUnicast()) {
			return "", errors.New("invalid Player discovery advertisement")
		}
		return strings.TrimSuffix(origin, "/"), nil
	}
	return "", nil
}

func invalidHostnameCharacter(char rune) bool {
	return (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-'
}

func validAdvertisement(name, origin string) bool {
	return name != "" && len(name) <= 63 && utf8.ValidString(name) && strings.TrimSpace(name) == name && !strings.ContainsFunc(name, unicode.IsControl) && len(origin) <= 240 && strings.TrimSpace(origin) == origin
}

func validDiscoveryHost(host string, ip net.IP) bool {
	if ip != nil {
		return !ip.IsUnspecified()
	}
	return validHostname(host)
}
