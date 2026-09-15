package server

import "github.com/MikeO7/kinosail/packages/mcpgateway"

var mcpReadRoutes = routeSet(
	"GET /api/v1", "GET /api/v1/openapi.json", "GET /api/v1/me", "GET /api/v1/library",
	"GET /api/v1/items/{id}", "GET /api/v1/items/{id}/playback", "GET /api/v1/history",
	"GET /api/v1/audio/{id}/queue", "GET /api/v1/books/{id}/reader", "GET /api/v1/books/{id}/reader/progress",
	"GET /api/v1/playlists", "GET /api/v1/playlists/{name}",
	"GET /api/v1/collections", "GET /api/v1/collections/{name}",
	"GET /api/v1/actor", "GET /api/v1/shows", "GET /api/v1/shows/{id}", "GET /api/v1/albums", "GET /api/v1/albums/{id}",
	"GET /api/v1/watch-rooms/{id}",
)

var mcpWriteRoutes = routeSet(
	"PUT /api/v1/items/{id}/progress", "PUT /api/v1/books/{id}/reader/progress", "DELETE /api/v1/items/{id}/continue-watching", "PUT /api/v1/items/{id}/list",
	"POST /api/v1/playlists", "POST /api/v1/smart-playlists", "DELETE /api/v1/playlists/{name}",
	"PUT /api/v1/playlists/{name}/items/{id}", "PUT /api/v1/playlists/{name}/order", "POST /api/v1/watch-rooms", "PUT /api/v1/me/language",
)

var mcpManageRoutes = routeSet(
	"GET /api/v1/activity", "GET /api/v1/agent-connections", "GET /api/v1/backups", "GET /api/v1/configuration",
	"GET /api/v1/diagnostics", "GET /api/v1/hardware", "GET /api/v1/maintenance", "GET /api/v1/marker-analysis",
	"GET /api/v1/metrics", "GET /api/v1/remote-access", "GET /api/v1/settings", "GET /api/v1/updates", "GET /api/v1/viewing-syncs",
	"DELETE /api/v1/agent-connections/{id}", "DELETE /api/v1/viewing-syncs/{id}",
	"PUT /api/v1/items/{id}/metadata", "POST /api/v1/items/{id}/metadata/refresh",
	"POST /api/v1/metadata/bulk",
	"PUT /api/v1/items/{id}/markers", "DELETE /api/v1/items/{id}/markers/{type}", "POST /api/v1/marker-analysis",
	"POST /api/v1/collections", "DELETE /api/v1/collections/{name}", "PUT /api/v1/collections/{name}/items/{id}",
	"PUT /api/v1/configuration/{key}", "DELETE /api/v1/configuration/{key}", "POST /api/v1/libraries", "DELETE /api/v1/libraries",
	"PUT /api/v1/settings/server", "PUT /api/v1/settings/navigation", "PUT /api/v1/settings/onboarding", "PUT /api/v1/settings/playback", "PUT /api/v1/settings/transcoder", "PUT /api/v1/settings/subtitles",
	"PUT /api/v1/settings/scans", "PUT /api/v1/settings/dlna", "PUT /api/v1/settings/jellyfin", "PUT /api/v1/settings/trusted-https", "POST /api/v1/settings/trusted-https/validate", "POST /api/v1/settings/trusted-https/test", "PUT /api/v1/settings/updates", "DELETE /api/v1/settings/trusted-https",
	"POST /api/v1/transcoder/test", "POST /api/v1/tasks/{task}", "POST /api/v1/backups", "POST /api/v1/backups/verify",
	"POST /api/v1/viewing-imports/preview", "POST /api/v1/viewing-imports/{id}/apply",
	"POST /api/v1/viewing-syncs", "POST /api/v1/viewing-syncs/{id}/run", "POST /api/v1/updates", "POST /api/v1/updates/check",
)

var mcpBlockedRoutes = routeSet(
	"DELETE /api/v1/cast/sessions/{id}", "DELETE /api/v1/items/{id}/bookmarks/{bookmark}", "DELETE /api/v1/items/{id}/playback-preferences",
	"DELETE /api/v1/api-keys/{id}", "DELETE /api/v1/devices/{id}", "DELETE /api/v1/downloads/{id}", "DELETE /api/v1/media-shares/{id}", "DELETE /api/v1/passkeys/{id}",
	"DELETE /api/v1/me/mfa", "DELETE /api/v1/me/oidc", "DELETE /api/v1/me/saml", "DELETE /api/v1/profiles/{id}",
	"DELETE /api/v1/remote-access/kill", "DELETE /api/v1/remote-access/wireguard", "DELETE /api/v1/session", "DELETE /api/v1/sessions",
	"GET /api/v1/activity/export", "GET /api/v1/agent-connections/certificate", "GET /api/v1/api-keys", "GET /api/v1/backup",
	"GET /api/v1/cast/sessions/{id}", "GET /api/v1/devices", "GET /api/v1/downloads", "GET /api/v1/downloads/{id}", "GET /api/v1/downloads/{id}/file", "GET /api/v1/downloads/{id}/manifest", "GET /api/v1/downloads/identity", "GET /api/v1/events", "GET /api/v1/items/{id}/bookmarks", "GET /api/v1/items/{id}/download-tracks", "GET /api/v1/items/{id}/playback-preferences", "GET /api/v1/items/{id}/watch-progress", "GET /api/v1/me/media-preferences", "GET /api/v1/media-shares", "GET /api/v1/passkeys",
	"GET /api/v1/me/oidc/link", "GET /api/v1/me/saml/link", "GET /api/v1/profiles",
	"GET /api/v1/session/oidc", "GET /api/v1/session/saml", "GET /api/v1/supporter", "GET /api/v1/supporter/certificate.svg",
	"GET /api/v1/supporter/display", "PUT /api/v1/supporter/display",
	"GET /api/v1/supporter/certificates/patron-order.svg", "GET /api/v1/supporter/certificates/living-standard.svg", "GET /api/v1/watch-rooms/{id}/events",
	"GET /api/v1/home-assistant", "GET /api/v1/home-assistant/library", "GET /api/v1/home-assistant/players",
	"POST /api/v1/api-keys", "POST /api/v1/cast/devices/scan", "POST /api/v1/cast/sessions/{id}/commands", "POST /api/v1/items/{id}/bookmarks", "POST /api/v1/items/{id}/cast", "POST /api/v1/items/{id}/downloads", "POST /api/v1/items/{id}/playback-events", "POST /api/v1/me/mfa/setup",
	"POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish",
	"POST /api/v1/passkeys/register/begin", "POST /api/v1/passkeys/register/finish",
	"POST /api/v1/media-shares", "POST /api/v1/media-shares/claim", "POST /api/v1/profiles", "POST /api/v1/quick-connect", "POST /api/v1/quick-connect/token", "POST /api/v1/quick-connect/cancel", "GET /api/v1/quick-connect/pending", "GET /api/v1/quick-connect/{code}", "POST /api/v1/quick-connect/{code}",
	"POST /api/v1/remote-access/kill", "POST /api/v1/remote-access/wireguard",
	"POST /api/v1/session", "POST /api/v1/setup", "POST /api/v1/supporter/activate",
	"POST /api/v1/home-assistant/pair", "POST /api/v1/home-assistant/pairings", "POST /api/v1/home-assistant/playback/{id}", "POST /api/v1/home-assistant/players/{id}/commands", "POST /api/v1/home-assistant/token",
	"PUT /api/v1/home-assistant/players/{id}", "PUT /api/v1/items/{id}/playback-preferences", "PUT /api/v1/items/{id}/progress/sync", "PUT /api/v1/me/media-preferences", "PUT /api/v1/me/mfa", "PUT /api/v1/profiles/{id}", "PUT /api/v1/profiles/{id}/password", "PUT /api/v1/settings/home-assistant", "PUT /api/v1/settings/mfa", "PUT /api/v1/settings/session-timeouts",
)

type mcpRoutePolicy struct{}

func (mcpRoutePolicy) Allows(pattern string, access mcpgateway.AccessClass) bool {
	switch access {
	case mcpgateway.ReadAccess:
		return mcpReadRoutes[pattern]
	case mcpgateway.WriteAccess:
		return mcpWriteRoutes[pattern]
	case mcpgateway.ManageAccess:
		return mcpManageRoutes[pattern]
	default:
		return false
	}
}
