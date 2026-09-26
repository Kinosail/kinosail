package server

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

const reviewedRouteInventorySHA256 = "22aa8a76b5eb17f7279becbc9183c82ebe64c9f96ab2708716efa13096d9f87b"

var explicitlyAnonymousRoutes = routeSet(
	"GET /static/public-login.js", "POST /auth/quick-connect", "POST /auth/quick-connect/token", "POST /auth/quick-connect/cancel",
	"GET /healthz", "GET /manifest.webmanifest", "GET /service-worker.js", "GET /offline", "GET /favicon.ico",
	"GET /static/subtitle-inspector.js", "GET /static/subtitle-inspector.css", "GET /static/htmx.min.js", "GET /static/hls.min.js", "GET /static/player.js", "GET /static/downloads.js", "GET /static/pwa.js", "GET /static/main.kinosail.bundle.js", "GET /static/theme.js", "GET /static/manrope.woff2", "GET /static/app.css", "GET /static/supporter.js", "GET /static/supporter.css", "GET /static/supporter/badges/{file}",
	"GET /static/icon.svg", "GET /static/icon-192.png", "GET /static/icon-512.png", "GET /static/icon-maskable-512.png", "GET /static/apple-touch-icon.png", "GET /static/cinema-backdrop.jpg", "GET /static/passkeys.js",
	"GET /share", "GET /static/media-share.js",
	"GET /login", "POST /login", "GET /setup", "GET /language", "POST /setup", "POST /language", "POST /api/v1/session", "POST /api/v1/setup",
	"GET /api/v1/home-assistant", "POST /api/v1/home-assistant/pair", "POST /api/v1/home-assistant/token",
	"GET /api/v1/session/oidc", "GET /login/oidc", "GET /login/oidc/callback", "GET /login/mfa", "POST /login/mfa",
	"GET /api/v1/session/saml", "GET /login/saml", "GET /login/saml/metadata",
	"POST /login/saml/acs",
	"GET /.well-known/oauth-protected-resource/mcp", "GET /mcp", "DELETE /mcp", "POST /mcp",
	"GET /.well-known/oauth-authorization-server", "POST /oauth/register", "POST /oauth/token", "POST /oauth/revoke",
	"POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish", "POST /auth/passkeys/login/begin", "POST /auth/passkeys/login/finish",
	"POST /api/v1/quick-connect", "POST /api/v1/quick-connect/token",
	"GET /System/Info/Public", "GET /system/info/public", "GET /QuickConnect/Enabled", "GET /QuickConnect/Initiate", "POST /QuickConnect/Initiate", "GET /QuickConnect/Connect",
	"GET /Users/Public", "GET /Branding/Configuration", "POST /Users/AuthenticateByName", "POST /Users/AuthenticateWithQuickConnect",
)

var explicitCapabilityRoutes = routeSet(
	"GET /Videos/{id}/{stream}", "GET /Audio/{id}/{stream}", "GET /Videos/{id}/{source}/Subtitles/{index}/{stream}",
	"GET /dlna/{token}/device.xml", "GET /dlna/{token}/content.xml", "POST /dlna/{token}/content", "GET /dlna/{token}/media/{id}", "HEAD /dlna/{token}/media/{id}",
	"POST /api/v1/media-shares/claim", "GET /share/items", "GET /share/media/{id}", "HEAD /share/media/{id}",
	"GET /home-assistant/media/{id}",
)

var localAnonymousJellyfinRoutes = routeSet(
	"GET /Items/{id}/Images/{type}", "GET /Items/{id}/Images/{type}/{index}",
)

var featureGatedRoutes = routeSet(
	"GET /api/v1/home-assistant/library", "GET /api/v1/home-assistant/players", "GET /home-assistant/authorize", "POST /home-assistant/authorize",
	"POST /api/v1/home-assistant/pairings", "POST /api/v1/home-assistant/playback/{id}",
	"POST /api/v1/home-assistant/players/{id}/commands", "PUT /api/v1/home-assistant/players/{id}",
)

var ownerOnlyRoutes = routeSet(
	"GET /settings/management", "GET /api/v1/management-access", "POST /api/v1/management-access", "DELETE /api/v1/management-access",
	"POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices",
	"POST /settings/management/enable", "POST /settings/management/disable", "POST /settings/management/devices", "POST /settings/management/devices/revoke",
	"GET /subtitles/inspect/{id}", "GET /api/v1/subtitle-library/{id}/draft", "POST /api/v1/subtitle-library/{id}/draft", "GET /api/v1/subtitle-library/{id}/inspect", "GET /api/v1/subtitle-library/{id}/export",
	"POST /api/v1/subtitle-library/{id}/preview", "POST /api/v1/subtitle-library/{id}/apply", "POST /api/v1/subtitle-library/{id}/audio",
	"DELETE /api/v1/agent-connections/{id}", "DELETE /api/v1/api-keys/{id}", "DELETE /api/v1/collections/{name}", "DELETE /api/v1/configuration/{key}", "DELETE /api/v1/devices/{id}",
	"DELETE /api/v1/items/{id}/markers/{type}", "DELETE /api/v1/libraries", "DELETE /api/v1/profiles/{id}",
	"DELETE /api/v1/settings/trusted-https", "DELETE /api/v1/sessions", "DELETE /api/v1/viewing-syncs/{id}",
	"DELETE /api/v1/remote-access/kill",
	"DELETE /api/v1/media-shares/{id}",
	"GET /api/v1/activity", "GET /api/v1/activity/export", "GET /api/v1/agent-connections", "GET /api/v1/agent-connections/certificate", "GET /api/v1/api-keys", "GET /api/v1/backup", "GET /api/v1/backups", "GET /api/v1/configuration", "GET /api/v1/devices",
	"GET /api/v1/diagnostics", "GET /api/v1/hardware", "GET /api/v1/maintenance", "GET /api/v1/marker-analysis", "GET /api/v1/metrics",
	"GET /api/v1/profiles", "GET /api/v1/remote-access", "GET /api/v1/settings", "GET /api/v1/subtitle-library", "GET /api/v1/updates", "GET /api/v1/viewing-syncs",
	"GET /api/v1/home-assistant/library", "GET /api/v1/home-assistant/players",
	"GET /api/v1/supporter", "GET /api/v1/supporter/certificates/{family}", "GET /supporter",
	"GET /api/v1/media-shares", "GET /settings/media-shares", "GET /settings/remote-readiness",
	"GET /metadata/bulk",
	"GET /Auth/Keys", "GET /Users", "POST /Auth/Keys",
	"GET /home-assistant/authorize", "GET /onboarding", "GET /onboarding/connection", "GET /onboarding/finish", "GET /onboarding/household", "GET /onboarding/migrate", "GET /settings", "GET /settings/agent-connections", "GET /settings/backup", "GET /settings/backups", "GET /settings/configuration", "GET /settings/diagnostics.json", "GET /settings/metrics", "GET /settings/subtitles/cleanup", "GET /settings/system",
	"POST /api/v1/api-keys", "POST /api/v1/backups", "POST /api/v1/backups/verify", "POST /api/v1/collections", "POST /api/v1/metadata/bulk",
	"POST /api/v1/items/{id}/metadata/refresh", "POST /api/v1/items/{id}/subtitles", "POST /api/v1/libraries",
	"POST /api/v1/subtitle-library/fetch-wanted", "POST /api/v1/subtitle-library/maintain", "POST /api/v1/subtitle-library/{id}/fetch", "POST /api/v1/subtitle-library/{id}/replacement", "POST /api/v1/subtitle-library/{id}/restore", "POST /api/v1/subtitle-providers/test",
	"POST /api/v1/subtitles/cleanup", "POST /api/v1/subtitles/cleanup/preview",
	"POST /api/v1/marker-analysis", "POST /api/v1/profiles", "POST /api/v1/tasks/{task}",
	"POST /api/v1/media-shares",
	"POST /api/v1/home-assistant/pairings", "POST /api/v1/home-assistant/playback/{id}", "POST /api/v1/home-assistant/players/{id}/commands",
	"POST /api/v1/remote-access/kill",
	"POST /api/v1/transcoder/test",
	"POST /api/v1/supporter/activate", "POST /supporter/activate",
	"POST /api/v1/viewing-imports/preview", "POST /api/v1/viewing-imports/{id}/apply", "POST /api/v1/viewing-syncs", "POST /api/v1/viewing-syncs/{id}/run",
	"POST /collection/{name}/delete", "POST /collection/{name}/{id}", "POST /collection/{name}/items/{id}", "POST /collections", "POST /markers/{id}", "POST /markers/{id}/remove",
	"POST /metadata/bulk", "POST /metadata/{id}", "POST /metadata/{id}/refresh", "POST /onboarding/household", "POST /onboarding/jellyfin", "POST /onboarding/subtitles/language", "POST /onboarding/subtitles/libraries", "POST /onboarding/subtitles/libraries/remove", "POST /onboarding/subtitles/scans", "POST /onboarding/trusted-https", "POST /onboarding/updates", "POST /onboarding/updates/check", "POST /onboarding/viewing-imports/apply", "POST /onboarding/viewing-imports/preview", "POST /scan", "POST /subtitles/{id}/fetch",
	"POST /subtitles/manage/fetch-wanted", "POST /subtitles/manage/maintain", "POST /subtitles/manage/{id}/fetch", "POST /subtitles/manage/{id}/replacement", "POST /subtitles/manage/{id}/restore", "POST /subtitles/providers/test",
	"POST /settings/agent-connections/revoke", "POST /settings/api-keys", "POST /settings/api-keys/revoke", "POST /settings/backups/verify", "POST /settings/cache/clear",
	"POST /settings/configuration", "POST /settings/configuration/reset", "POST /settings/dlna", "POST /settings/encrypted-backup", "POST /settings/jellyfin",
	"POST /home-assistant/authorize", "POST /settings/home-assistant", "POST /settings/home-assistant/pair", "POST /onboarding/home-assistant",
	"POST /settings/libraries", "POST /settings/libraries/remove", "POST /settings/marker-analysis", "POST /settings/playback",
	"POST /settings/mfa",
	"POST /settings/session-timeouts",
	"POST /settings/media-shares", "POST /settings/media-shares/{id}/revoke", "POST /settings/onboarding",
	"POST /settings/profiles", "POST /settings/profiles/password", "POST /settings/profiles/permissions", "POST /settings/profiles/remove",
	"POST /settings/remote/enable", "POST /settings/remote/kill",
	"POST /settings/trusted-https", "POST /settings/trusted-https/disable",
	"POST /settings/navigation", "POST /settings/scans", "POST /settings/server", "POST /settings/sessions/device", "POST /settings/sessions/revoke", "POST /settings/subtitles", "POST /settings/subtitles/cleanup", "POST /settings/subtitles/subsource", "POST /settings/subtitles/subsource/reset",
	"POST /settings/tasks/maintain", "POST /settings/tasks/metadata", "POST /settings/tasks/scan", "POST /settings/transcoder", "POST /settings/transcoder/test", "POST /settings/updates", "POST /settings/updates/check",
	"POST /api/v1/updates", "POST /api/v1/updates/check",
	"POST /settings/viewing-imports/apply", "POST /settings/viewing-imports/preview", "POST /settings/viewing-syncs", "POST /settings/viewing-syncs/remove", "POST /settings/viewing-syncs/run",
	"PUT /api/v1/collections/{name}/items/{id}", "PUT /api/v1/configuration/{key}", "PUT /api/v1/items/{id}/markers", "PUT /api/v1/items/{id}/metadata",
	"PUT /api/v1/profiles/{id}", "PUT /api/v1/profiles/{id}/password", "PUT /api/v1/settings/dlna",
	"PUT /api/v1/settings/home-assistant", "PUT /api/v1/settings/jellyfin", "PUT /api/v1/settings/mfa", "PUT /api/v1/settings/navigation", "PUT /api/v1/settings/onboarding", "PUT /api/v1/settings/playback", "PUT /api/v1/settings/scans", "PUT /api/v1/settings/server", "PUT /api/v1/settings/session-timeouts", "PUT /api/v1/settings/subtitles", "PUT /api/v1/settings/transcoder", "PUT /api/v1/settings/trusted-https", "PUT /api/v1/settings/updates",
)

var expectedLibraryScopeRoutes = routeSet(
	"GET /api/v1", "GET /api/v1/openapi.json", "GET /api/v1/library", "GET /api/v1/subtitle-library", "GET /api/v1/items/{id}", "GET /api/v1/items/{id}/playback", "GET /api/v1/items/{id}/watch-progress",
	"GET /api/v1/history", "GET /api/v1/audio/{id}/queue", "GET /api/v1/books/{id}/reader", "GET /api/v1/books/{id}/reader/progress", "GET /api/v1/playlists", "GET /api/v1/playlists/{name}",
	"GET /api/v1/collections", "GET /api/v1/collections/{name}", "GET /api/v1/shows", "GET /api/v1/shows/{id}", "GET /api/v1/albums", "GET /api/v1/albums/{id}",
	"GET /api/v1/watch-rooms/{id}", "GET /art/{id}", "GET /backdrop/{id}", "GET /person/{id}/{person}", "GET /System/Info", "GET /Library/MediaFolders", "GET /Users", "GET /Users/Me", "GET /Users/{id}",
	"GET /UserViews", "GET /Users/{user}/Views", "GET /Items", "GET /Users/{user}/Items", "GET /Items/Latest", "GET /Users/{user}/Items/Latest", "GET /Persons", "GET /Items/{id}", "GET /Users/{user}/Items/{id}", "GET /Shows/{id}/Seasons", "GET /Shows/{id}/Episodes", "GET /Items/{id}/Images/{type}", "GET /Items/{id}/Images/{type}/{index}",
)

var expectedWriteScopeRoutes = routeSet(
	"PUT /api/v1/items/{id}/progress", "PUT /api/v1/books/{id}/reader/progress", "DELETE /api/v1/items/{id}/continue-watching", "PUT /api/v1/items/{id}/list",
	"POST /api/v1/playlists", "POST /api/v1/smart-playlists", "DELETE /api/v1/playlists/{name}",
	"PUT /api/v1/playlists/{name}/items/{id}", "PUT /api/v1/playlists/{name}/order", "POST /api/v1/watch-rooms", "POST /api/v1/items/{id}/playback-events",
)

var expectedStreamScopeRoutes = routeSet(
	"GET /media/{id}", "GET /hls/{id}/{file...}", "GET /hls/{id}/audio/{track}/{file...}",
	"GET /Videos/{id}/{stream...}",
	"GET /subtitle/{id}", "GET /subtitle/{id}/{track}", "GET /subtitle/{id}/embedded/{stream}", "GET /subtitles/{id}/{language}", "GET /trickplay/{id}/{second}",
	"GET /read/{id}/asset/{asset...}", "GET /read/{id}/file", "GET /api/v1/events", "GET /api/v1/watch-rooms/{id}/events",
)

var expectedDownloadScopeRoutes = routeSet(
	"GET /download/{id}", "POST /api/v1/items/{id}/downloads", "GET /api/v1/downloads", "GET /api/v1/downloads/{id}",
	"GET /api/v1/downloads/{id}/file", "DELETE /api/v1/downloads/{id}",
)

var sessionOnlyAPIRoutes = routeSet(
	"GET /api/v1/backup", "POST /api/v1/management-access", "DELETE /api/v1/management-access", "POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices",
	"DELETE /api/v1/session", "GET /api/v1/me/oidc/link", "DELETE /api/v1/me/oidc", "GET /api/v1/me/saml/link", "DELETE /api/v1/me/saml",
	"POST /api/v1/me/mfa/setup", "PUT /api/v1/me/mfa", "DELETE /api/v1/me/mfa",
	"GET /api/v1/passkeys", "DELETE /api/v1/passkeys/{id}", "POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish", "POST /api/v1/quick-connect/{code}",
)

func TestEveryRegisteredRouteHasReviewedAnonymousAccess(t *testing.T) {
	authRoutesContract().ReviewedAnonymousAccess(t, 462, reviewedRouteInventorySHA256, func(t *testing.T, data string) http.Handler {
		return New(Config{DataDir: data, RequireAuth: true, Configuration: jellyfinRouteConfiguration(t, data)})
	})
}

func TestEveryProtectedRouteAcceptsOwnerAndRejectsInvalidOrRevokedSessions(t *testing.T) {
	authRoutesContract().EveryProtectedRouteAcceptsOwnerAndRejectsInvalidOrRevokedSessions(t)
}

func TestEveryProtectedRouteEnforcesOwnerAndViewerRoles(t *testing.T) {
	authRoutesContract().EveryProtectedRouteEnforcesOwnerAndViewerRoles(t)
}

func TestEveryProtectedRouteEnforcesViewerSchedule(t *testing.T) {
	authRoutesContract().EveryProtectedRouteEnforcesViewerSchedule(t)
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
		LocalAnonymous:      localAnonymousJellyfinRoutes,
		AssertCapability:    assertCapabilityRoute,
	}
}
