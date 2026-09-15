package identitycore

// PublicBootstrapRouteAllowed is the closed internet allowlist for routes that
// bypass normal Viewer authorization. Media handlers still validate capabilities;
// sign-in handlers still enforce remote Viewer eligibility and strong sign-in.
// Local integration routes must never become public merely by being registered.
func PublicBootstrapRouteAllowed(pattern string) bool {
	switch pattern {
	case "GET /static/public-login.js", "POST /auth/quick-connect", "POST /auth/quick-connect/token", "POST /auth/quick-connect/cancel":
		return true
	case "GET /manifest.webmanifest", "GET /service-worker.js", "GET /offline", "GET /favicon.ico",
		"GET /static/htmx.min.js", "GET /static/hls.min.js", "GET /static/player.js", "GET /static/downloads.js", "GET /static/pwa.js",
		"GET /static/main.kinosail.bundle.js", "GET /static/theme.js", "GET /static/quick-connect.js", "GET /static/connect.js",
		"GET /static/app.css", "GET /static/supporter.js", "GET /static/supporter.css", "GET /static/manrope.woff2", "GET /static/icon.svg",
		"GET /static/subtitle-inspector.js", "GET /static/subtitle-inspector.css", "GET /static/supporter/badges/{file...}",
		"GET /static/media-share.js", "GET /share", "GET /share/items", "GET /share/media/{id}", "HEAD /share/media/{id}", "POST /api/v1/media-shares/claim",
		"GET /static/icon-192.png", "GET /static/icon-512.png", "GET /static/icon-maskable-512.png", "GET /static/apple-touch-icon.png", "GET /static/cinema-backdrop.jpg", "GET /static/passkeys.js",
		"GET /login", "POST /login", "GET /language", "POST /language", "POST /api/v1/session",
		"GET /api/v1/session/oidc", "GET /login/oidc", "GET /login/oidc/callback",
		"GET /api/v1/session/saml", "GET /login/saml", "GET /login/saml/metadata", "POST /login/saml/acs", "GET /login/mfa", "POST /login/mfa",
		"POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish", "POST /auth/passkeys/login/begin", "POST /auth/passkeys/login/finish",
		"POST /api/v1/quick-connect", "POST /api/v1/quick-connect/token", "POST /api/v1/quick-connect/cancel",
		"GET /System/Info/Public", "GET /system/info/public", "GET /QuickConnect/Enabled", "GET /QuickConnect/Initiate", "POST /QuickConnect/Initiate", "GET /QuickConnect/Connect", "GET /Users/Public",
		"GET /Branding/Configuration", "POST /Users/AuthenticateByName", "POST /Users/AuthenticateWithQuickConnect",
		"GET /Videos/{id}/{stream}", "GET /Videos/{id}/{stream...}", "GET /Audio/{id}/{stream}", "GET /Videos/{id}/{source}/Subtitles/{index}/{stream}":
		return true
	}
	return false
}
