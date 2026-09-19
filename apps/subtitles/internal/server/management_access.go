package server

import (
	"net"
	"net/http"
	"net/url"

	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/owneraccess"
)

var managementView = newLocalizedTemplate("management", owneraccess.View)

func newOwnerAccess(config Config, auth *authentication) *owneraccess.Manager {
	return owneraccess.OpenRuntime(owneraccess.RuntimeConfig{
		Directory: config.DataDir, Origin: config.AuthURL, Lifecycle: config.Lifecycle,
		DuckDNSDomain: config.Configuration.String("remote.duckdns_domain"),
		DuckDNSToken:  config.Configuration.String("remote.duckdns_token"), Profile: auth.profiles.byID,
		TrustedCertificate: config.TrustedHTTPS.Certificate, InternetCertificate: config.InternetAccess.Certificate,
	})
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
