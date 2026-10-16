package server

import (
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/mcpgateway"
)

var publicRoutes = map[string]bool{
	"GET /healthz": true, "POST /mcp": true, "GET /mcp": true, "DELETE /mcp": true,
	"GET /.well-known/oauth-protected-resource/mcp": true, "GET /.well-known/oauth-authorization-server": true,
	"POST /oauth/register": true, "POST /oauth/token": true, "POST /oauth/revoke": true,
	"GET /manifest.webmanifest": true, "GET /service-worker.js": true, "GET /offline": true, "GET /favicon.ico": true,
	"GET /static/subtitle-inspector.js": true, "GET /static/subtitle-inspector.css": true,
	"GET /static/htmx.min.js": true, "GET /static/pwa.js": true,
	"GET /static/main.kinosail.bundle.js": true, "GET /static/theme.js": true, "GET /static/app.css": true, "GET /static/supporter.js": true, "GET /static/supporter.css": true, "GET /static/supporter/badges/{file}": true, "GET /static/manrope.woff2": true, "GET /static/tmdb-logo.svg": true, "GET /static/icon.svg": true,
	"GET /static/icon-192.png": true, "GET /static/icon-512.png": true, "GET /static/icon-maskable-512.png": true, "GET /static/apple-touch-icon.png": true, "GET /static/cinema-backdrop.jpg": true, "GET /static/passkeys.js": true,
	"GET /login": true, "POST /login": true, "GET /setup": true, "GET /language": true, "POST /setup": true, "POST /language": true,
	"POST /api/v1/session": true, "POST /api/v1/setup": true, "GET /api/v1/session/oidc": true,
	"GET /api/v1/session/saml": true, "GET /login/saml": true, "GET /login/saml/metadata": true,
	"POST /login/saml/acs": true,
	"GET /login/oidc":      true, "GET /login/oidc/callback": true, "GET /login/mfa": true, "POST /login/mfa": true,
	"POST /api/v1/passkeys/login/begin": true, "POST /api/v1/passkeys/login/finish": true,
	"POST /auth/passkeys/login/begin": true, "POST /auth/passkeys/login/finish": true,
}

func init() {
	for _, pattern := range mcpgateway.Patterns() {
		if pattern != "GET /oauth/authorize" && pattern != "POST /oauth/authorize" {
			publicRoutes[pattern] = true
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
