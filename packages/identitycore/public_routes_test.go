package identitycore

import (
	"strings"
	"testing"
)

func TestPublicBootstrapRoutesDenyIntegrationsAndUnknownPatterns(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{
		"", "GET /scim/v2/Users", "POST /scim/v2/Users", "PATCH /scim/v2/Users/{id}",
		"GET /scim/v2/ServiceProviderConfig", "GET /mcp", "POST /oauth/token", "GET /settings",
		"POST /api/v1/setup", "POST /api/v1/profiles", "POST /api/v1/passkeys/register/begin",
		"GET /auth/quick-connect", "POST /auth/quick-connect/approve", "POST /auth/quick-connect/extra",
		"GET /static/{file...}", "POST /static/app.css", "GET /static/app.css/extra",
		"GET /login/extra", "GET /LOGIN", "GET /login\x00", " GET /login", "GET /login ",
		"GET /new-integration", strings.Repeat("x", 8192),
	} {
		if PublicBootstrapRouteAllowed(pattern) {
			t.Errorf("internet bootstrap allowed %q", pattern)
		}
	}
}

func TestPublicBootstrapPreservesSignInAndMediaEntryPoints(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{
		"GET /login", "POST /api/v1/passkeys/login/begin", "POST /api/v1/passkeys/login/finish",
		"POST /api/v1/quick-connect", "POST /api/v1/quick-connect/token",
		"GET /static/public-login.js", "POST /auth/quick-connect", "POST /auth/quick-connect/token", "POST /auth/quick-connect/cancel",
		"GET /api/v1/session/oidc", "POST /login/saml/acs", "POST /Users/AuthenticateWithQuickConnect",
		"GET /static/app.css", "GET /static/manrope.woff2", "GET /Videos/{id}/{stream...}",
		"GET /Audio/{id}/{stream}", "GET /share/media/{id}", "POST /api/v1/media-shares/claim",
	} {
		if !PublicBootstrapRouteAllowed(pattern) {
			t.Errorf("internet bootstrap blocked %q", pattern)
		}
	}
}
