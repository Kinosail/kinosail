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
	localCompatibilityRoute
)

var publicRoutes = map[string]routeAccess{
	"GET /static/public-login.js": publicRoute, "POST /auth/quick-connect": publicRoute, "POST /auth/quick-connect/token": publicRoute, "POST /auth/quick-connect/cancel": publicRoute,
	"GET /healthz": publicRoute, "POST /mcp": publicRoute, "GET /mcp": publicRoute, "DELETE /mcp": publicRoute,
	"GET /.well-known/oauth-protected-resource/mcp": publicRoute, "GET /.well-known/oauth-authorization-server": publicRoute,
	"POST /oauth/register": publicRoute, "POST /oauth/token": publicRoute, "POST /oauth/revoke": publicRoute,
	"GET /manifest.webmanifest": publicRoute, "GET /service-worker.js": publicRoute, "GET /offline": publicRoute, "GET /favicon.ico": publicRoute,
	"GET /static/subtitle-inspector.js": publicRoute, "GET /static/subtitle-inspector.css": publicRoute,
	"GET /static/htmx.min.js": publicRoute, "GET /static/hls.min.js": publicRoute, "GET /static/player.js": publicRoute, "GET /static/downloads.js": publicRoute, "GET /static/pwa.js": publicRoute,
	"GET /static/main.kinosail.bundle.js": publicRoute, "GET /static/theme.js": publicRoute, "GET /static/app.css": publicRoute, "GET /static/supporter.js": publicRoute, "GET /static/supporter.css": publicRoute, "GET /static/manrope.woff2": publicRoute, "GET /static/icon.svg": publicRoute,
	"GET /static/media-share.js": publicRoute, "GET /share": publicRoute, "GET /share/items": capabilityRoute, "GET /share/media/{id}": capabilityRoute, "HEAD /share/media/{id}": capabilityRoute, "POST /api/v1/media-shares/claim": publicRoute,
	"GET /static/icon-192.png": publicRoute, "GET /static/icon-512.png": publicRoute, "GET /static/icon-maskable-512.png": publicRoute, "GET /static/apple-touch-icon.png": publicRoute, "GET /static/cinema-backdrop.jpg": publicRoute, "GET /static/passkeys.js": publicRoute,
	"GET /login": publicRoute, "POST /login": publicRoute, "GET /setup": publicRoute, "GET /language": publicRoute, "POST /setup": publicRoute, "POST /language": publicRoute,
	"POST /api/v1/session": publicRoute, "POST /api/v1/setup": publicRoute, "GET /api/v1/session/oidc": publicRoute,
	"GET /api/v1/session/saml": publicRoute, "GET /login/saml": publicRoute, "GET /login/saml/metadata": publicRoute,
	"POST /login/saml/acs":       publicRoute,
	"GET /api/v1/home-assistant": publicRoute, "POST /api/v1/home-assistant/pair": publicRoute, "POST /api/v1/home-assistant/token": publicRoute, "GET /home-assistant/media/{id}": capabilityRoute,
	"GET /login/oidc": publicRoute, "GET /login/oidc/callback": publicRoute, "GET /login/mfa": publicRoute, "POST /login/mfa": publicRoute,
	"POST /api/v1/passkeys/login/begin": publicRoute, "POST /api/v1/passkeys/login/finish": publicRoute,
	"POST /auth/passkeys/login/begin": publicRoute, "POST /auth/passkeys/login/finish": publicRoute,
	"POST /api/v1/quick-connect": publicRoute, "POST /api/v1/quick-connect/token": publicRoute,
	"GET /System/Info/Public": publicRoute, "GET /system/info/public": publicRoute, "GET /QuickConnect/Enabled": publicRoute,
	"GET /QuickConnect/Initiate": publicRoute, "POST /QuickConnect/Initiate": publicRoute, "GET /QuickConnect/Connect": publicRoute, "GET /Users/Public": publicRoute,
	"GET /Branding/Configuration": publicRoute, "POST /Users/AuthenticateByName": publicRoute, "POST /Users/AuthenticateWithQuickConnect": publicRoute,
	"GET /Items/{id}/Images/{type}": localCompatibilityRoute, "GET /Items/{id}/Images/{type}/{index}": localCompatibilityRoute,
	"GET /Videos/{id}/{stream}": capabilityRoute, "GET /Videos/{id}/{stream...}": capabilityRoute, "GET /Audio/{id}/{stream}": capabilityRoute,
	"GET /Videos/{id}/{source}/Subtitles/{index}/{stream}": capabilityRoute,
	"GET /dlna/{token}/device.xml":                         capabilityRoute, "GET /dlna/{token}/content.xml": capabilityRoute, "POST /dlna/{token}/content": capabilityRoute,
	"GET /dlna/{token}/media/{id}": capabilityRoute, "HEAD /dlna/{token}/media/{id}": capabilityRoute,
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
	case "GET /healthz", "GET /setup", "POST /setup", "POST /api/v1/setup", "POST /mcp", "GET /mcp", "DELETE /mcp",
		"GET /api/v1/home-assistant", "POST /api/v1/home-assistant/pair", "POST /api/v1/home-assistant/token", "GET /home-assistant/authorize", "POST /home-assistant/authorize", "GET /home-assistant/media/{id}",
		"GET /.well-known/oauth-protected-resource/mcp", "GET /.well-known/oauth-authorization-server", "POST /oauth/register", "POST /oauth/token", "POST /oauth/revoke",
		"GET /oauth/authorize", "POST /oauth/authorize", "GET /quick-connect", "POST /quick-connect",
		"POST /api/v1/quick-connect/{code}", "POST /QuickConnect/Authorize", "GET /Items/{id}/Images/{type}", "GET /Items/{id}/Images/{type}/{index}", "GET /dlna/{token}/device.xml", "GET /dlna/{token}/content.xml", "POST /dlna/{token}/content", "GET /dlna/{token}/media/{id}", "HEAD /dlna/{token}/media/{id}":
		return true
	}
	return false
}

func publicViewerRouteAllowed(pattern string) bool {
	if pattern == "GET /api/v1" || pattern == "GET /api/v1/openapi.json" {
		return false
	}
	if apiLibraryRoutes[pattern] || apiWriteRoutes[pattern] || apiStreamRoutes[pattern] || apiDownloadRoutes[pattern] {
		return true
	}
	if jellyfinLibraryRoutes[pattern] {
		return true
	}
	switch pattern {
	case "GET /api/v1/me", "PUT /api/v1/me/language", "DELETE /api/v1/session", "POST /logout",
		"GET /{$}", "GET /album/{id}", "GET /book/{id}", "GET /collection/{name}",
		"GET /offline-downloads", "GET /playlist/{name}", "GET /read/{id}", "GET /room/{id}", "GET /show/{id}", "GET /watch-together/{id}", "GET /watch/{id}",
		"POST /continue-watching/{id}/remove", "POST /list/{id}", "POST /offline-downloads/{id}/remove", "POST /offline/{id}",
		"POST /playlist/{name}/delete", "POST /playlist/{name}/items/{id}", "POST /playlist/{name}/order", "POST /playlist/{name}/{id}", "POST /playlists", "POST /progress/{id}", "POST /read/{id}/progress", "POST /smart-playlists", "POST /watch-together", "POST /watched/{id}",
		"DELETE /UserFavoriteItems/{id}", "DELETE /UserPlayedItems/{id}", "DELETE /Users/{user}/PlayedItems/{id}",
		"GET /Items", "GET /Items/Latest", "GET /Items/{id}", "GET /Items/{id}/Download", "GET /Items/{id}/File", "GET /Items/{id}/Images/{type}", "GET /Items/{id}/Images/{type}/{index}", "GET /Items/{id}/PlaybackInfo", "GET /Videos/{id}/{stream...}",
		"GET /MediaSegments/{id}", "GET /Playback/BitrateTest", "GET /Shows/NextUp", "GET /Shows/{id}/Episodes", "GET /Shows/{id}/Seasons", "GET /System/Info",
		"GET /UserItems/Resume", "GET /UserItems/{id}/UserData", "GET /UserViews", "GET /Users/Me", "GET /Users/{id}", "GET /Users/{user}/Items", "GET /Users/{user}/Items/Latest", "GET /Users/{user}/Items/{id}", "GET /Users/{user}/Views",
		"POST /Items/{id}/PlaybackInfo", "POST /Sessions/Capabilities", "POST /Sessions/Capabilities/Full", "POST /Sessions/Logout", "POST /Sessions/Playing", "POST /Sessions/Playing/Progress", "POST /Sessions/Playing/Stopped",
		"POST /UserFavoriteItems/{id}", "POST /UserItems/{id}/UserData", "POST /UserPlayedItems/{id}", "POST /Users/{user}/PlayedItems/{id}":
		return true
	}
	return false
}
