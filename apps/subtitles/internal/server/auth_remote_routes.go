package server

import (
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/mcpgateway"
	"github.com/MikeO7/kinosail/packages/scim"
)

type routeAccess uint8

const (
	publicRoute routeAccess = iota + 1
	capabilityRoute
)

var publicRoutes = map[string]routeAccess{
	"GET /healthz": publicRoute, "POST /mcp": publicRoute, "GET /mcp": publicRoute, "DELETE /mcp": publicRoute,
	"GET /.well-known/oauth-protected-resource/mcp": publicRoute, "GET /.well-known/oauth-authorization-server": publicRoute,
	"POST /oauth/register": publicRoute, "POST /oauth/token": publicRoute, "POST /oauth/revoke": publicRoute,
	"GET /manifest.webmanifest": publicRoute, "GET /service-worker.js": publicRoute, "GET /offline": publicRoute, "GET /favicon.ico": publicRoute,
	"GET /static/subtitle-inspector.js": publicRoute, "GET /static/subtitle-inspector.css": publicRoute,
	"GET /static/htmx.min.js": publicRoute, "GET /static/pwa.js": publicRoute,
	"GET /static/main.kinosail.bundle.js": publicRoute, "GET /static/theme.js": publicRoute, "GET /static/app.css": publicRoute, "GET /static/supporter.js": publicRoute, "GET /static/supporter.css": publicRoute, "GET /static/supporter/badges/{file}": publicRoute, "GET /static/manrope.woff2": publicRoute, "GET /static/tmdb-logo.svg": publicRoute, "GET /static/icon.svg": publicRoute,
	"GET /static/icon-192.png": publicRoute, "GET /static/icon-512.png": publicRoute, "GET /static/icon-maskable-512.png": publicRoute, "GET /static/apple-touch-icon.png": publicRoute, "GET /static/cinema-backdrop.jpg": publicRoute, "GET /static/passkeys.js": publicRoute,
	"GET /login": publicRoute, "POST /login": publicRoute, "GET /setup": publicRoute, "GET /language": publicRoute, "POST /setup": publicRoute, "POST /language": publicRoute,
	"POST /api/v1/session": publicRoute, "POST /api/v1/setup": publicRoute, "GET /api/v1/session/oidc": publicRoute,
	"GET /api/v1/session/saml": publicRoute, "GET /login/saml": publicRoute, "GET /login/saml/metadata": publicRoute,
	"POST /login/saml/acs":       publicRoute,
	"GET /api/v1/home-assistant": publicRoute, "POST /api/v1/home-assistant/pair": publicRoute, "POST /api/v1/home-assistant/token": publicRoute, "GET /home-assistant/media/{id}": capabilityRoute,
	"GET /login/oidc": publicRoute, "GET /login/oidc/callback": publicRoute, "GET /login/mfa": publicRoute, "POST /login/mfa": publicRoute,
	"POST /api/v1/passkeys/login/begin": publicRoute, "POST /api/v1/passkeys/login/finish": publicRoute,
	"POST /auth/passkeys/login/begin": publicRoute, "POST /auth/passkeys/login/finish": publicRoute,
}

func init() {
	for _, pattern := range scim.Patterns() {
		publicRoutes[pattern] = publicRoute
	}
	for _, pattern := range mcpgateway.Patterns() {
		if pattern != "GET /oauth/authorize" && pattern != "POST /oauth/authorize" {
			publicRoutes[pattern] = publicRoute
		}
	}
}

func remotePublicRouteDenied(pattern string) bool {
	if _, bypassesViewer := publicRoutes[pattern]; bypassesViewer && !identitycore.PublicBootstrapRouteAllowed(pattern) {
		return true
	}
	switch pattern {
	case "POST /settings/management/enable", "POST /settings/management/disable", "POST /settings/management/devices", "POST /settings/management/devices/revoke",
		"GET /settings/management", "GET /api/v1/management-access", "POST /api/v1/management-access", "DELETE /api/v1/management-access", "POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices":
		return true
	case "GET /healthz", "GET /setup", "POST /setup", "POST /api/v1/setup", "POST /mcp", "GET /mcp", "DELETE /mcp",
		"GET /.well-known/oauth-protected-resource/mcp", "GET /.well-known/oauth-authorization-server", "POST /oauth/register", "POST /oauth/token", "POST /oauth/revoke",
		"GET /oauth/authorize", "POST /oauth/authorize":
		return true
	}
	return false
}

func publicViewerRouteAllowed(pattern string) bool {
	if pattern == "GET /api/v1" || pattern == "GET /api/v1/openapi.json" {
		return false
	}
	if apiLibraryRoutes[pattern] || apiStreamRoutes[pattern] {
		return true
	}
	switch pattern {
	case "GET /api/v1/me", "PUT /api/v1/me/language", "DELETE /api/v1/session", "POST /logout", "GET /{$}":
		return true
	}
	return false
}
