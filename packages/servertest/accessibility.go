package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// AgentConnectionsExposeAccessibleConsentAndSettingsLandmarks verifies agent setup guidance.
func AgentConnectionsExposeAccessibleConsentAndSettingsLandmarks(t *testing.T, handler http.Handler, origin string) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/settings/agent-connections", nil))
	for _, value := range []string{`href="#main"`, `id="main"`, `<h1>Connect Codex and other AI agents</h1>`, `docker exec -i kinosail kinosail mcp-stdio`, `codex mcp add kinosail-http --url ` + origin + `/mcp`, `codex mcp login kinosail-http`} {
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), value) {
			t.Fatalf("agent connection settings missing %q: %d %q", value, response.Code, response.Body.String())
		}
	}
}

// AccountJourneysExposeMainLandmarks verifies the unauthenticated account pages.
func AccountJourneysExposeMainLandmarks(t *testing.T, handler http.Handler) {
	t.Helper()
	for _, path := range []string{"/setup", "/login", "/quick-connect"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "<main") {
			t.Fatalf("%s landmark = %d %q", path, response.Code, response.Body.String())
		}
	}
}
