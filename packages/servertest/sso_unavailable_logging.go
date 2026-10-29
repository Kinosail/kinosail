package servertest

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/federation"
	"github.com/MikeO7/kinosail/packages/scim"
)

func AssertUnconfiguredSSOLogsWarnAndProviderOutageLogsError(t *testing.T, fixture OIDCFixture) {
	output := &lockedLog{}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	unconfigured := fixture.NewHandler(t.TempDir(), federation.OIDCConfig{}, scim.Config{})
	routes := []string{"/login/oidc", "/api/v1/session/oidc", "/login/saml", "/api/v1/session/saml"}
	for _, path := range routes {
		assertSSOStatus(t, unconfigured, path, http.StatusServiceUnavailable)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(provider.Close)
	outage := fixture.NewHandler(t.TempDir(), federation.OIDCConfig{Issuer: provider.URL, ClientID: "kinosail", ClientSecret: "test-client-secret", RedirectURL: "http://localhost/login/oidc/callback"}, scim.Config{})
	assertSSOStatus(t, outage, "/api/v1/session/oidc", http.StatusServiceUnavailable)
	records := output.records("request")
	levels := make(map[string][]any)
	for _, record := range records {
		if route, _ := record["route"].(string); record["status"] == float64(http.StatusServiceUnavailable) {
			levels[route] = append(levels[route], record["level"])
		}
	}
	for _, path := range []string{"GET /login/oidc", "GET /login/saml", "GET /api/v1/session/saml"} {
		if len(levels[path]) != 1 || levels[path][0] != "WARN" {
			t.Errorf("unconfigured %s logged %v, want one WARN", path, levels[path])
		}
	}
	if session := levels["GET /api/v1/session/oidc"]; len(session) != 2 || session[0] != "WARN" || session[1] != "ERROR" {
		t.Errorf("OIDC session levels = %v, want unconfigured WARN then provider outage ERROR", session)
	}
}

func assertSSOStatus(t *testing.T, handler http.Handler, path string, status int) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	if response.Code != status {
		t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
	}
}
