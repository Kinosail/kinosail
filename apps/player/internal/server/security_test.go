package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHTTPSecurityContract(t *testing.T) {
	servertest.HTTPSecurity(t, func(config servertest.SecurityConfig[configuration.Snapshot]) http.Handler {
		return server.New(server.Config{DataDir: config.DataDir, RequireAuth: true, ProxyToken: config.ProxyToken, AuthURL: config.AuthURL, Configuration: config.Configuration})
	}, configuration.Load, "38127", "38128")
}

func TestSessionTokensAreNeverAcceptedInQueryStrings(t *testing.T) {
	t.Parallel()
	servertest.SessionTokenSecurity(t, newJellyfinServer(t, server.Config{RequireAuth: true}), apiCall, mustJSON, assertAPIBody, testTOTP)
}

func TestExactOriginTakesPriorityOverCrossSiteNavigationMetadata(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		origin string
		want   int
	}{
		"exact origin": {origin: "https://kinosail.test", want: http.StatusUnauthorized},
		"other origin": {origin: "https://attacker.test", want: http.StatusForbidden},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://kinosail.test/login", strings.NewReader("name=probe&password=invalid"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Sec-Fetch-Site", "cross-site")
			response := httptest.NewRecorder()
			server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, AuthURL: "https://kinosail.test"}).ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("POST /login = %d %q", response.Code, response.Body.String())
			}
		})
	}
}
