package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHTTPSecurityContract(t *testing.T) {
	servertest.HTTPSecurity(t, func(config servertest.SecurityConfig[configuration.Snapshot]) http.Handler {
		return server.New(server.Config{DataDir: config.DataDir, RequireAuth: true, ProxyToken: config.ProxyToken, AuthURL: config.AuthURL, Configuration: config.Configuration})
	}, configuration.Load, "38128", "38129")
}

func TestSessionTokensAreNeverAcceptedInQueryStrings(t *testing.T) {
	t.Parallel()
	servertest.SessionTokenSecurity(t, newJellyfinServer(t, server.Config{RequireAuth: true}), apiCall, mustJSON, assertAPIBody, testTOTP)
}

func TestSameOriginChangesAreAcceptedWhenFetchSiteIsStale(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test/setup", strings.NewReader("name=Owner&password=owner-password"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Origin", "https://kinosail.test")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true}).ServeHTTP(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("same-origin setup with stale fetch metadata = %d %q", response.Code, response.Body.String())
	}
}
