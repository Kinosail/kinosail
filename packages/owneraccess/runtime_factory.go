package owneraccess

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

// RuntimeConfig contains the application-owned inputs needed to attach private
// management. Certificate sources are callbacks so this package does not know
// which public or LAN listener supplied them.
type RuntimeConfig struct {
	Directory, Origin           string
	DuckDNSDomain, DuckDNSToken string
	Lifecycle                   context.Context
	Profile                     func(string) (identitycore.Profile, bool)
	TrustedCertificate          func(string) *tls.Certificate
	InternetCertificate         func(string) *tls.Certificate
}

// OpenRuntime assembles private management around the application boundary.
// It returns nil when management is not configured or cannot be opened; the
// application remains usable and reports that state through its management UI.
func OpenRuntime(config RuntimeConfig) *Manager {
	if config.Directory == "" {
		return nil
	}
	tunnel, domain, token := tunnelCertificate(config)
	manager, err := Open(Config{
		Directory: config.Directory,
		Origin:    config.Origin,
		Profile:   config.Profile,
		MaintainEndpoint: func(ctx context.Context, endpoint string) error {
			hostname, _, _ := net.SplitHostPort(endpoint)
			if domain != "" && hostname == domain+".duckdns.org" {
				return remoteaccess.UpdateDNS(ctx, domain, token)
			}
			return nil
		},
		Certificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			for _, source := range []func(string) *tls.Certificate{
				certificateSource(tunnel), config.TrustedCertificate, config.InternetCertificate,
			} {
				if source == nil {
					continue
				}
				if certificate := source(hello.ServerName); certificate != nil {
					return certificate, nil
				}
			}
			return nil, errors.New("trusted HTTPS is not ready")
		},
	})
	if err != nil {
		slog.Error("private management unavailable", "error", err)
		return nil
	}
	if tunnel != nil && config.Lifecycle != nil {
		go tunnel.Run(config.Lifecycle)
	}
	return manager
}

func tunnelCertificate(config RuntimeConfig) (*trustedhttps.Manager, string, string) {
	domain, token := config.DuckDNSDomain, config.DuckDNSToken
	origin, err := url.Parse(config.Origin)
	if err != nil || domain == "" || token == "" || origin.Hostname() != domain+".duckdns.org" {
		return nil, domain, token
	}
	tunnel, err := trustedhttps.NewTunnelCertificate(domain, token, Address, filepath.Join(config.Directory, "owner-https"))
	if err != nil {
		return nil, domain, token
	}
	return tunnel, domain, token
}

func certificateSource(manager *trustedhttps.Manager) func(string) *tls.Certificate {
	return func(serverName string) *tls.Certificate {
		if manager == nil {
			return nil
		}
		return manager.Certificate(serverName)
	}
}
