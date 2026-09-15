package server

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/owneraccess"
	"github.com/MikeO7/kinosail/packages/remoteaccess"
	"github.com/MikeO7/kinosail/packages/trustedhttps"
)

var managementView = newLocalizedTemplate("management", owneraccess.View)

func newOwnerAccess(config Config, auth *authentication) *owneraccess.Manager {
	if config.DataDir == "" {
		return nil
	}
	var tunnelCertificate *trustedhttps.Manager
	domain, token := config.Configuration.String("remote.duckdns_domain"), config.Configuration.String("remote.duckdns_token")
	if origin, err := url.Parse(config.AuthURL); err == nil && origin.Hostname() == domain+".duckdns.org" && domain != "" && token != "" {
		tunnelCertificate, err = trustedhttps.NewTunnelCertificate(domain, token, owneraccess.Address, filepath.Join(config.DataDir, "owner-https"))
		if err == nil && config.Lifecycle != nil {
			go tunnelCertificate.Run(config.Lifecycle)
		}
	}
	manager, err := owneraccess.Open(owneraccess.Config{
		Directory: config.DataDir, Origin: config.AuthURL, Profile: auth.profiles.byID,
		MaintainEndpoint: func(ctx context.Context, endpoint string) error {
			hostname, _, _ := net.SplitHostPort(endpoint)
			if domain != "" && hostname == domain+".duckdns.org" {
				return remoteaccess.UpdateDNS(ctx, domain, token)
			}
			return nil
		},
		Certificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			if tunnelCertificate != nil {
				if cert := tunnelCertificate.Certificate(hello.ServerName); cert != nil {
					return cert, nil
				}
			}
			if config.TrustedHTTPS != nil {
				if cert := config.TrustedHTTPS.Certificate(hello.ServerName); cert != nil {
					return cert, nil
				}
			}
			if config.InternetAccess != nil {
				if cert := config.InternetAccess.Certificate(hello.ServerName); cert != nil {
					return cert, nil
				}
			}
			return nil, errors.New("trusted HTTPS is not ready")
		},
	})
	if err != nil {
		slog.Error("private management unavailable", "error", err)
		return nil
	}
	return manager
}

func (auth *authentication) freshOwner(next http.Handler) http.Handler {
	return identitycore.OwnerSensitive(next, identitycore.OwnerConfig{
		Identity: func(r *http.Request) (bool, bool) {
			p := currentViewer(r)
			return p.Owner && p.Secured() && p.ID != "local-owner" && !p.APIKey, false
		},
		Managed: func(*http.Request) bool { return false }, RecentlyAuthenticated: auth.profiles.recentlyAuthenticated,
		AuthenticationError: authenticationErrorRequest, StepUpPath: stepUpLoginPath, Error: localizedError, JSON: writeJSON,
	})
}

func registerOwnerAccess(mux *http.ServeMux, auth *authentication, manager *owneraccess.Manager, origin string) {
	h := owneraccess.HTTP{Manager: manager, OwnerID: func(r *http.Request) string { return currentViewer(r).ID }, JSON: writeJSON, Error: localizedError}
	mux.Handle("GET /settings/management", auth.owner(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if manager == nil {
			localizedError(w, r, "private management is unavailable", http.StatusServiceUnavailable)
			return
		}
		view := struct {
			owneraccess.Status
			PublicPort string
			HostPort   string
			AtHome     bool
		}{Status: manager.Status(), HostPort: "51822", AtHome: owneraccess.ProfileID(r) == ""}
		_, view.PublicPort, _ = net.SplitHostPort(view.Endpoint)
		if !view.Enabled {
			if u, err := url.Parse(origin); err == nil && u.Hostname() != "" {
				view.Endpoint = u.Hostname() + ":51822"
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = managementView.Execute(w, r, view)
	})))
	mux.Handle("GET /api/v1/management-access", auth.owner(http.HandlerFunc(h.Status)))
	mux.Handle("POST /api/v1/management-access", auth.freshOwner(h.Change("enable", true)))
	mux.Handle("DELETE /api/v1/management-access", auth.freshOwner(h.Change("disable", true)))
	mux.Handle("POST /api/v1/management-access/devices", auth.freshOwner(h.Change("pair", true)))
	mux.Handle("DELETE /api/v1/management-access/devices", auth.freshOwner(h.Change("revoke", true)))
	mux.Handle("POST /settings/management/enable", auth.freshOwner(h.Change("enable", false)))
	mux.Handle("POST /settings/management/disable", auth.freshOwner(h.Change("disable", false)))
	mux.Handle("POST /settings/management/devices", auth.freshOwner(h.Change("pair", false)))
	mux.Handle("POST /settings/management/devices/revoke", auth.freshOwner(h.Change("revoke", false)))
}
