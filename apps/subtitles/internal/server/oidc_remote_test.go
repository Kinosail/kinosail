package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestPublicInternetListenerExposesOIDCProviderRoutes(t *testing.T) {
	t.Parallel()
	public := server.Remote(server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}))
	for _, path := range []string{"/api/v1/session/oidc", "/login/oidc", "/login/oidc/callback?code=value&state=value"} {
		if response := serveRequest(public, requestWithBody(t, http.MethodGet, path, "")); response.Code == http.StatusNotFound {
			t.Fatalf("public OIDC route %s was hidden: %d %q", path, response.Code, response.Body.String())
		}
	}
}
