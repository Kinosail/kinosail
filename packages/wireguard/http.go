package wireguard

import (
	"errors"
	"net/http"
)

// Profile is the authorization data needed for a Viewer pairing.
type Profile struct {
	ID    string
	Owner bool
}

// ProfileLookup resolves pairing targets without coupling the protocol to an app store.
type ProfileLookup interface {
	WireGuardProfile(string) (Profile, bool)
}

// HTTPConfig supplies one application's JSON and localized error writers.
type HTTPConfig struct {
	Manager     *Manager
	Profiles    ProfileLookup
	ReadJSON    func(http.ResponseWriter, *http.Request, any) bool
	WriteJSON   func(http.ResponseWriter, any, int)
	APIError    func(http.ResponseWriter, error, int)
	APINotFound func(http.ResponseWriter)
	WebError    func(http.ResponseWriter, *http.Request, string, int)
}

// NewHTTP binds one application's storage and response conventions.
func NewHTTP(manager *Manager, profiles ProfileLookup, read func(http.ResponseWriter, *http.Request, any) bool, write func(http.ResponseWriter, any, int), apiError func(http.ResponseWriter, error, int), notFound func(http.ResponseWriter), webError func(http.ResponseWriter, *http.Request, string, int)) HTTPConfig {
	return HTTPConfig{manager, profiles, read, write, apiError, notFound, webError}
}

// CreateAPI returns the strict JSON pairing handler.
func (config HTTPConfig) CreateAPI() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ Label, ProfileID string }
		if !config.ReadJSON(writer, request, &input) {
			return
		}
		if config.Manager == nil {
			config.APINotFound(writer)
			return
		}
		profile, found := config.Profiles.WireGuardProfile(input.ProfileID)
		if !found || profile.Owner {
			config.APIError(writer, errors.New("WireGuard pairing requires a Viewer Profile"), http.StatusBadRequest)
			return
		}
		pairing, err := config.Manager.Pair(input.Label, profile.ID)
		if err != nil {
			config.APIError(writer, err, http.StatusBadRequest)
			return
		}
		config.WriteJSON(writer, map[string]string{"profile": pairing.ViewerConfig}, http.StatusCreated)
	}
}

// RevokeAPI returns the strict JSON revocation handler.
func (config HTTPConfig) RevokeAPI() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ PublicKey string }
		if !config.ReadJSON(writer, request, &input) {
			return
		}
		if config.Manager == nil || config.Manager.Revoke(input.PublicKey) != nil {
			config.APINotFound(writer)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

// RevokeWeb returns the localized form revocation handler.
func (config HTTPConfig) RevokeWeb() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if config.Manager == nil || config.Manager.Revoke(request.FormValue("publicKey")) != nil {
			config.WebError(writer, request, "WireGuard Viewer not found", http.StatusNotFound)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

// CreateWeb returns the localized Viewer profile download handler.
func (config HTTPConfig) CreateWeb() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if config.Manager == nil {
			config.WebError(writer, request, "Verified Direct Connection is not configured", http.StatusNotFound)
			return
		}
		profile, found := config.Profiles.WireGuardProfile(request.FormValue("profileId"))
		if !found || profile.Owner {
			config.WebError(writer, request, "WireGuard pairing requires a Viewer Profile", http.StatusBadRequest)
			return
		}
		pairing, err := config.Manager.Pair(request.FormValue("label"), profile.ID)
		if err != nil {
			config.WebError(writer, request, err.Error(), http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "application/x-wireguard-profile")
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-viewer.conf"`)
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte(pairing.ViewerConfig))
	}
}
