package server

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

const reviewedRouteInventorySHA256 = "788586950483fe8ebd9be387ad56c0e45719d5430f0685df9a4a51ef57e8ada9"

var explicitlyAnonymousRoutes = routeSet(
	"GET /healthz", "GET /manifest.webmanifest", "GET /service-worker.js", "GET /offline", "GET /favicon.ico",
	"GET /static/subtitle-inspector.js", "GET /static/subtitle-inspector.css", "GET /static/htmx.min.js", "GET /static/pwa.js", "GET /static/main.kinosail.bundle.js", "GET /static/theme.js", "GET /static/manrope.woff2", "GET /static/app.css", "GET /static/supporter.js", "GET /static/supporter.css", "GET /static/supporter/badges/{file}",
	"GET /static/tmdb-logo.svg", "GET /static/icon.svg", "GET /static/icon-192.png", "GET /static/icon-512.png", "GET /static/icon-maskable-512.png", "GET /static/apple-touch-icon.png", "GET /static/cinema-backdrop.jpg", "GET /static/passkeys.js",
	"GET /login", "POST /login", "GET /setup", "GET /language", "POST /setup", "POST /language", "POST /api/v1/session", "POST /api/v1/setup",
	"GET /api/v1/session/oidc", "GET /login/oidc", "GET /login/oidc/callback", "GET /login/mfa", "POST /login/mfa",
	"GET /api/v1/session/saml", "GET /login/saml", "GET /login/saml/metadata",
	"POST /login/saml/acs",
	"GET /.well-known/oauth-protected-resource/mcp", "GET /mcp", "DELETE /mcp", "POST /mcp",
	"GET /.well-known/oauth-authorization-server", "POST /oauth/register", "POST /oauth/token", "POST /oauth/revoke",
	"POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish", "POST /auth/passkeys/login/begin", "POST /auth/passkeys/login/finish",
)

var explicitCapabilityRoutes = routeSet()

var featureGatedRoutes = routeSet()

var ownerOnlyRoutes = routeSet(
	"GET /{$}", "GET /media/{id}", "POST /api/v1/subtitle-operations", "GET /api/v1/subtitle-operations/{operation}", "GET /api/v1/subtitle-operations/{operation}/result",
	"GET /settings/management", "GET /api/v1/management-access", "POST /api/v1/management-access", "DELETE /api/v1/management-access",
	"POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices",
	"POST /settings/management/enable", "POST /settings/management/disable", "POST /settings/management/devices", "POST /settings/management/devices/revoke",
	"GET /subtitles/inspect/{id}", "GET /api/v1/subtitle-library/{id}/draft", "POST /api/v1/subtitle-library/{id}/draft", "GET /api/v1/subtitle-library/{id}/inspect", "GET /api/v1/subtitle-library/{id}/export",
	"POST /api/v1/subtitle-library/{id}/preview", "POST /api/v1/subtitle-library/{id}/apply", "POST /api/v1/subtitle-library/{id}/audio",
	"DELETE /api/v1/agent-connections/{id}", "DELETE /api/v1/api-keys/{id}", "DELETE /api/v1/configuration/{key}", "DELETE /api/v1/devices/{id}",
	"DELETE /api/v1/libraries",
	"DELETE /api/v1/settings/trusted-https", "DELETE /api/v1/sessions",
	"GET /api/v1/activity", "GET /api/v1/activity/export", "GET /api/v1/agent-connections", "GET /api/v1/agent-connections/certificate", "GET /api/v1/api-keys", "GET /api/v1/backup", "GET /api/v1/backups", "GET /api/v1/configuration", "GET /api/v1/devices",
	"GET /api/v1/diagnostics", "GET /api/v1/maintenance", "GET /api/v1/metrics",
	"GET /api/v1/settings", "GET /api/v1/subtitle-library", "GET /api/v1/updates",
	"GET /api/v1/supporter", "GET /api/v1/supporter/certificates/{family}", "GET /supporter",
	"GET /onboarding", "GET /onboarding/connection", "GET /onboarding/finish", "GET /settings", "GET /settings/agent-connections", "GET /settings/backup", "GET /settings/backups", "GET /settings/configuration", "GET /settings/diagnostics.json", "GET /settings/metrics", "GET /settings/subtitles/cleanup", "GET /settings/system",
	"POST /api/v1/api-keys", "POST /api/v1/backups", "POST /api/v1/backups/verify",
	"POST /api/v1/libraries",
	"POST /api/v1/subtitle-library/fetch-wanted", "POST /api/v1/subtitle-library/maintain", "POST /api/v1/subtitle-library/{id}/fetch", "POST /api/v1/subtitle-library/{id}/replacement", "POST /api/v1/subtitle-library/{id}/restore", "POST /api/v1/subtitle-providers/test",
	"POST /api/v1/subtitles/cleanup", "POST /api/v1/subtitles/cleanup/preview",
	"POST /api/v1/tasks/{task}",
	"POST /api/v1/supporter/activate", "POST /supporter/activate",
	"POST /onboarding/subtitles/language", "POST /onboarding/subtitles/libraries", "POST /onboarding/subtitles/libraries/remove", "POST /onboarding/subtitles/scans", "POST /scan",
	"POST /subtitles/manage/fetch-wanted", "POST /subtitles/manage/maintain", "POST /subtitles/manage/{id}/fetch", "POST /subtitles/manage/{id}/replacement", "POST /subtitles/manage/{id}/restore", "POST /subtitles/providers/test",
	"POST /settings/agent-connections/revoke", "POST /settings/api-keys", "POST /settings/api-keys/revoke", "POST /settings/backups/verify",
	"POST /settings/configuration", "POST /settings/configuration/reset", "POST /settings/encrypted-backup",
	"POST /settings/libraries", "POST /settings/libraries/remove",
	"POST /settings/mfa",
	"POST /settings/session-timeouts",
	"POST /settings/onboarding",
	"POST /settings/trusted-https", "POST /settings/trusted-https/disable",
	"POST /settings/scans", "POST /settings/server", "POST /settings/sessions/device", "POST /settings/sessions/revoke", "POST /settings/subtitles", "POST /settings/subtitles/cleanup", "POST /settings/subtitles/subsource", "POST /settings/subtitles/subsource/reset",
	"POST /settings/tasks/maintain", "POST /settings/tasks/metadata", "POST /settings/tasks/scan", "POST /settings/updates", "POST /settings/updates/check",
	"POST /api/v1/updates", "POST /api/v1/updates/check",
	"PUT /api/v1/configuration/{key}",
	"PUT /api/v1/settings/mfa", "PUT /api/v1/settings/onboarding", "PUT /api/v1/settings/scans", "PUT /api/v1/settings/server", "PUT /api/v1/settings/session-timeouts", "PUT /api/v1/settings/subtitles", "PUT /api/v1/settings/trusted-https", "PUT /api/v1/settings/updates",
)

var expectedLibraryScopeRoutes = routeSet(
	"GET /api/v1", "GET /api/v1/openapi.json", "GET /api/v1/library", "GET /api/v1/subtitle-library",
	"GET /art/{id}",
)

var expectedStreamScopeRoutes = routeSet(
	"GET /api/v1/events",
)

var sessionOnlyAPIRoutes = routeSet(
	"GET /api/v1/backup", "POST /api/v1/management-access", "DELETE /api/v1/management-access", "POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices",
	"DELETE /api/v1/session", "GET /api/v1/me/oidc/link", "DELETE /api/v1/me/oidc", "GET /api/v1/me/saml/link", "DELETE /api/v1/me/saml",
	"POST /api/v1/me/mfa/setup", "PUT /api/v1/me/mfa", "DELETE /api/v1/me/mfa",
	"GET /api/v1/passkeys", "DELETE /api/v1/passkeys/{id}", "POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish",
)

func TestEveryRegisteredRouteHasReviewedAnonymousAccess(t *testing.T) {
	authRoutesContract().ReviewedAnonymousAccess(t, 209, reviewedRouteInventorySHA256, func(t *testing.T, data string) http.Handler {
		return New(Config{DataDir: data, RequireAuth: true, Configuration: duckDNSRouteConfiguration(t, data)})
	})
}

func TestEveryProtectedRouteAcceptsOwnerAndRejectsInvalidOrRevokedSessions(t *testing.T) {
	authRoutesContract().EveryProtectedRouteAcceptsOwnerAndRejectsInvalidOrRevokedSessions(t)
}

func TestEveryProtectedRouteEnforcesExactAPIKeyScopes(t *testing.T) {
	authRoutesContract().EveryProtectedRouteEnforcesExactAPIKeyScopes(t)
}

func authRoutesContract() servertest.AuthRoutes {
	return servertest.AuthRoutes{
		Inventory:           registeredRouteInventory,
		New:                 newRouteAuthorizationServer,
		Login:               loginRouteProfile,
		Exercise:            exerciseRoute,
		Denied:              globalAuthenticationDenied,
		AuthorizationDenied: routeAuthorizationDenied,
		CreateProfile:       createRouteProfile,
		JSON:                routeJSON,
		AssertPolicyDenied:  assertViewerPolicyDenied,
		CreateKey:           createRouteAPIKey,
		Scopes:              expectedAPIKeyScopes,
		Anonymous:           explicitlyAnonymousRoutes,
		Capability:          explicitCapabilityRoutes,
		OwnerOnly:           ownerOnlyRoutes,
		FeatureGated:        featureGatedRoutes,
		AssertCapability:    assertCapabilityRoute,
	}
}
