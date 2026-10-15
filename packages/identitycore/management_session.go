package identitycore

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
)

type (
	managementDeviceKey struct{}
	managementDevice    struct{ profileID, publicKey string }
)

func WithManagementDevice(r *http.Request, profileID, publicKey string) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), managementDeviceKey{}, managementDevice{profileID, publicKey}))
}

func ManagementProfileID(r *http.Request) string {
	if r == nil {
		return ""
	}
	value, _ := r.Context().Value(managementDeviceKey{}).(managementDevice)
	return value.profileID
}

func ManagementDeviceKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	value, _ := r.Context().Value(managementDeviceKey{}).(managementDevice)
	return value.publicKey
}

func SessionMatchesRequest(s Session, r *http.Request) bool {
	if r == nil {
		return false
	}
	key := ManagementDeviceKey(r)
	if s.ManagementDevice == "" {
		return key == ""
	}
	return !RemoteRequest(r) && validManagementKey(s.ManagementDevice) && s.ManagementDevice == key && s.ProfileID == ManagementProfileID(r)
}

func validManagementKey(raw string) bool {
	key, err := base64.StdEncoding.DecodeString(raw)
	return err == nil && len(key) == 32 && base64.StdEncoding.EncodeToString(key) == raw
}

func (sessions *RequestSessions) CreateForRequest(r *http.Request, id, name string, browser, strong bool, channel string) (string, error) {
	if r == nil {
		return "", ErrInvalidConfig
	}
	key := ManagementDeviceKey(r)
	if key != "" {
		if channel != "" || id != ManagementProfileID(r) || !validManagementKey(key) {
			return "", ErrSessionKind
		}
		return sessions.core().create(id, name, browser, strong, "", nil, key)
	}
	return sessions.core().Create(id, name, browser, strong, channel)
}

func ManagementBootstrapRoute(pattern string) bool {
	if strings.HasPrefix(pattern, "GET /static/") {
		return true
	}
	switch pattern {
	case "GET /healthz", "GET /favicon.ico", "GET /manifest.webmanifest", "GET /service-worker.js", "GET /offline",
		"GET /login", "POST /login", "GET /login/mfa", "POST /login/mfa", "POST /api/v1/session",
		"GET /login/oidc", "GET /login/oidc/callback", "GET /api/v1/session/oidc",
		"GET /login/saml", "GET /login/saml/metadata", "POST /login/saml/acs", "GET /api/v1/session/saml",
		"POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish", "POST /auth/passkeys/login/begin", "POST /auth/passkeys/login/finish":
		return true
	default:
		return false
	}
}
