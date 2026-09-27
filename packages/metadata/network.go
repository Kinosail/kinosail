package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"net/url"
	pathpkg "path"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
	"github.com/MikeO7/kinosail/packages/privatefile"
)

func hardenedHTTPClient(timeout time.Duration) *http.Client {
	return outboundHTTPClient(timeout, allowedOutboundIP)
}

func localIntegrationHTTPClient(timeout time.Duration) *http.Client {
	return outboundHTTPClient(timeout, allowedIntegrationIP)
}

func outboundHTTPClient(timeout time.Duration, allowed func(net.IP) bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("outbound address is invalid")
		}
		addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil || len(addresses) == 0 {
			return nil, errors.New("outbound host could not be resolved")
		}
		for _, resolved := range addresses {
			if !allowed(resolved) {
				return nil, errors.New("outbound host resolved to a prohibited address")
			}
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(addresses[0].String(), port))
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func allowedOutboundIP(address net.IP) bool {
	return allowedIntegrationIP(address) && !address.IsLoopback() && !address.IsPrivate() && !cgnatIP(address)
}

func allowedIntegrationIP(address net.IP) bool {
	return address != nil && !address.IsUnspecified() && !address.IsMulticast() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast()
}

func cgnatIP(address net.IP) bool {
	value := address.To4()
	return value != nil && value[0] == 100 && value[1]&0xc0 == 0x40
}

func decodeExternalJSONStrict(reader interface{ Read([]byte) (int, error) }, maximum int64, target any) error {
	return httpguard.DecodeJSON(reader, maximum, target, true)
}

func decodeExternalJSON(reader interface{ Read([]byte) (int, error) }, maximum int64, target any) error {
	return httpguard.DecodeJSON(reader, maximum, target, false)
}

func saveJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return privatefile.WriteCache(path, data)
}

func validMetadataYear(value string) bool {
	if value == "" {
		return true
	}
	if len(value) != 4 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

func validProviderEndpoint(endpoint *url.URL, officialHost, path string) bool { //nolint:cyclop // Official HTTPS and loopback test endpoints share one fail-closed policy.
	if endpoint == nil || endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.Path != path {
		return false
	}
	if endpoint.Scheme == "https" && endpoint.Hostname() == officialHost && endpoint.Port() == "" {
		return true
	}
	ip := net.ParseIP(endpoint.Hostname())
	return endpoint.Scheme == "http" && endpoint.Port() != "" && (endpoint.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}

func validProviderBaseURL(raw string) bool { //nolint:cyclop // Explicit HTTPS integrations and loopback fixtures share one bounded URL grammar.
	if raw == "" || len(raw) > 2048 || strings.TrimSpace(raw) != raw {
		return false
	}
	endpoint, err := url.Parse(raw)
	if err != nil || endpoint.Opaque != "" || endpoint.User != nil || endpoint.Hostname() == "" || endpoint.RawQuery != "" || endpoint.Fragment != "" || endpoint.RawPath != "" || strings.Contains(endpoint.Path, `\`) || hasControlText(endpoint.Path) {
		return false
	}
	normalizedPath := strings.TrimRight(endpoint.Path, "/")
	if normalizedPath != "" && (!strings.HasPrefix(normalizedPath, "/") || pathpkg.Clean(normalizedPath) != normalizedPath) {
		return false
	}
	if endpoint.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(endpoint.Hostname())
	return endpoint.Scheme == "http" && endpoint.Port() != "" && (endpoint.Hostname() == "localhost" || ip != nil && ip.IsLoopback())
}

// ValidProviderBaseURL checks a configured metadata endpoint before it is saved.
func ValidProviderBaseURL(raw string) bool { return validProviderBaseURL(raw) }

// Configured reports whether the licensed metadata provider can be used safely.
func Configured(cache, token, baseURL, imageURL string) bool {
	return cache != "" && token != "" && validProviderBaseURL(baseURL) && validProviderBaseURL(imageURL)
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
