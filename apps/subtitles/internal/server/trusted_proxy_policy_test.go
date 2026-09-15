package server_test

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestTrustedProxyUsesPublicInternetPolicy(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true, ProxyToken: "proxy-capability"})
	public := trustedProxyHandler(handler)
	if response := serveRequest(public, requestWithBody(t, http.MethodPost, "/setup", "name=Attacker&password=attacker-password")); response.Code != http.StatusNotFound {
		t.Fatalf("trusted proxy setup = %d %q", response.Code, response.Body.String())
	}
	if response := apiCall(t, public, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Attacker", "password": "attacker-password"}); response.Code != http.StatusNotFound {
		t.Fatalf("trusted proxy API setup = %d %q", response.Code, response.Body.String())
	}
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	if response := serveRequest(public, requestWithCookieRequest(t, http.MethodGet, "/settings", "", owner)); response.Code != http.StatusForbidden {
		t.Fatalf("trusted proxy Owner settings = %d %q", response.Code, response.Body.String())
	}
	createdKey := requestWithCookie(t, handler, http.MethodPost, "/settings/api-keys", "name=Dashboard&scopes=library", owner)
	key := regexp.MustCompile(`ks_[A-Z2-7]+`).FindString(createdKey.Body.String())
	if createdKey.Code != http.StatusCreated || key == "" {
		t.Fatalf("create API key = %d %q", createdKey.Code, createdKey.Body.String())
	}
	if response := apiKeyRequest(t, public, "/api/v1/library", key); response.Code != http.StatusForbidden {
		t.Fatalf("trusted proxy API key = %d %q", response.Code, response.Body.String())
	}
	create := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&remote=true", owner)
	if response := serveRequest(handler, create); response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
	}
	login := requestWithBody(t, http.MethodPost, "/login", "name=Viewer&password=viewer-password")
	if response := serveRequest(public, login); response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "passkey or Quick Connect") {
		t.Fatalf("trusted proxy password login = %d %q", response.Code, response.Body.String())
	}
}

func trustedProxyPublicViewer(t *testing.T, handler http.Handler) (http.Handler, string) {
	t.Helper()
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	create := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&remote=true&transcode=true", owner)
	if response := serveRequest(handler, create); response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d %q", response.Code, response.Body.String())
	}
	viewer := signInTestProfile(t, handler, "/login", "name=Viewer&password=viewer-password")
	confirmTestFactor(t, handler, viewer, requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", viewer))
	public := trustedProxyHandler(handler)
	started := serveRequest(public, requestWithBody(t, http.MethodPost, "/api/v1/quick-connect", "device=Internet+TV"))
	var state struct{ Code, Secret string }
	if started.Code != http.StatusCreated || json.Unmarshal(started.Body.Bytes(), &state) != nil {
		t.Fatalf("start Quick Connect = %d %q", started.Code, started.Body.String())
	}
	if response := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+state.Code, "", viewer)); response.Code != http.StatusNoContent {
		t.Fatalf("approve Quick Connect = %d %q", response.Code, response.Body.String())
	}
	connected := serveRequest(public, requestWithBody(t, http.MethodPost, "/api/v1/quick-connect/token", "secret="+state.Secret))
	var session struct{ Token string }
	if connected.Code != http.StatusCreated || json.Unmarshal(connected.Body.Bytes(), &session) != nil || session.Token == "" {
		t.Fatalf("connect Quick Connect = %d %q", connected.Code, connected.Body.String())
	}
	return public, session.Token
}

func trustedProxyHandler(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		request.Header.Set("X-Kinosail-Proxy-Token", "proxy-capability")
		handler.ServeHTTP(writer, request)
	})
}
