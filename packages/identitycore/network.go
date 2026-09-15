package identitycore

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

// OutboundHTTPClient returns a redirect-disabled client whose resolved addresses must all pass policy.
func OutboundHTTPClient(timeout time.Duration, allowed func(net.IP) bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10_000_000_000, KeepAlive: 30_000_000_000}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, errors.New("outbound address is invalid")
		}
		addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, errors.New("outbound host could not be resolved")
		}
		resolved, err := AllowedResolvedAddress(addresses, allowed)
		if err != nil {
			return nil, err
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(resolved.String(), port))
	}
	return &http.Client{Timeout: timeout, Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

// AllowedResolvedAddress rejects an empty, partially denied, or unconfigured DNS result.
func AllowedResolvedAddress(addresses []net.IP, allowed func(net.IP) bool) (net.IP, error) {
	if len(addresses) == 0 {
		return nil, errors.New("outbound host returned no addresses")
	}
	if allowed == nil {
		return nil, ErrInvalidConfig
	}
	for _, address := range addresses {
		if !allowed(address) {
			return nil, errors.New("outbound host resolved to a prohibited address")
		}
	}
	return addresses[0], nil
}

// AllowedOutboundIP permits only public unicast integration addresses.
func AllowedOutboundIP(address net.IP) bool {
	return AllowedIntegrationIP(address) && !address.IsLoopback() && !address.IsPrivate() && !cgnatIP(address)
}

// AllowedIntegrationIP permits specified unicast addresses, including private networks.
func AllowedIntegrationIP(address net.IP) bool {
	return address != nil && !address.IsUnspecified() && !address.IsMulticast() && !address.IsLinkLocalUnicast() && !address.IsLinkLocalMulticast()
}

func cgnatIP(address net.IP) bool {
	value := address.To4()
	return value != nil && value[0] == 100 && value[1]&0xc0 == 0x40
}
