package server_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestPublicBrowserQuickConnectEndToEndBoundary(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	handler := server.New(server.Config{DataDir: data, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	disableTestMFA(t, handler, owner.Value)
	create := requestWithCookieRequest(t, http.MethodPost, "/settings/profiles", "name=Viewer&password=viewer-password&rating=all&libraries=all&remote=true", owner)
	if response := serveRequest(handler, create); response.Code != http.StatusSeeOther {
		t.Fatalf("create Viewer = %d", response.Code)
	}
	viewer := signInTestProfile(t, handler, "/login", "name=Viewer&password=viewer-password")
	enrollment := requestWithCookie(t, handler, http.MethodPost, "/account/mfa/setup", "", viewer)
	confirmTestFactor(t, handler, viewer, enrollment)
	public := server.Remote(handler)
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		response := serveRequest(public, requestWithBody(t, method, "/login?next=https://attacker.example", ""))
		if response.Code != http.StatusOK || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("public login = %d", response.Code)
		}
		if method == http.MethodGet && (!strings.Contains(response.Body.String(), "Get a sign-in code") || strings.Contains(response.Body.String(), `name="password"`) || strings.Contains(response.Body.String(), "attacker.example")) {
			t.Fatalf("public login offered an invalid flow: %s", response.Body.String())
		}
	}
	asset := serveRequest(public, requestWithBody(t, http.MethodGet, "/static/public-login.js?v=1", ""))
	if asset.Code != http.StatusOK || !strings.Contains(asset.Body.String(), "/auth/quick-connect") {
		t.Fatalf("public login script = %d", asset.Code)
	}
	call := func(path string, cookie *http.Cookie) *httptest.ResponseRecorder {
		request := requestWithBody(t, http.MethodPost, path, "")
		request.Header.Del("Content-Type")
		request.TLS = &tls.ConnectionState{}
		request.Header.Set("Origin", "https://"+request.Host)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		if cookie != nil {
			request.AddCookie(cookie)
		}
		return serveRequest(public, request)
	}
	started := call("/auth/quick-connect", nil)
	var state struct{ Code string }
	if started.Code != http.StatusCreated || json.Unmarshal(started.Body.Bytes(), &state) != nil || state.Code == "" || len(started.Result().Cookies()) != 1 {
		t.Fatalf("browser start = %d %s", started.Code, started.Body.String())
	}
	pending := started.Result().Cookies()[0]
	if response := call("/auth/quick-connect/token", pending); response.Code != http.StatusAccepted {
		t.Fatalf("pending browser = %d %s", response.Code, response.Body.String())
	}
	for _, c := range []*http.Cookie{owner, viewer} {
		remoteApproval := serveRequest(public, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+state.Code, "", c))
		if remoteApproval.Code != http.StatusNotFound {
			t.Fatalf("public approval allowed = %d", remoteApproval.Code)
		}
	}
	approved := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/api/v1/quick-connect/"+state.Code, "", viewer))
	if approved.Code != http.StatusNoContent {
		t.Fatalf("local approval = %d %s", approved.Code, approved.Body.String())
	}
	connected := call("/auth/quick-connect/token", pending)
	if connected.Code != http.StatusNoContent {
		t.Fatalf("browser sign-in = %d %s", connected.Code, connected.Body.String())
	}
	var session *http.Cookie
	for _, cookie := range connected.Result().Cookies() {
		if cookie.Name == "__Host-kinosail_session" {
			session = cookie
		}
	}
	if session == nil || !session.HttpOnly || !session.Secure || session.MaxAge != 28800 {
		t.Fatalf("browser session cookie = %#v", session)
	}
	library := serveRequest(public, requestWithCookieRequest(t, http.MethodGet, "/api/v1/library", "", session))
	if library.Code != http.StatusOK {
		t.Fatalf("public library = %d %s", library.Code, library.Body.String())
	}
	for _, path := range []string{"/settings", "/account", "/api/v1/profiles", "/api/v1/backup", "/scim/v2/Users"} {
		response := serveRequest(public, requestWithCookieRequest(t, http.MethodGet, path, "", session))
		if response.Code != http.StatusNotFound {
			t.Fatalf("public browser reached %s = %d", path, response.Code)
		}
	}
	id := storedProfileID(t, data, "Viewer")
	revoked := serveRequest(handler, requestWithCookieRequest(t, http.MethodPost, "/settings/profiles/permissions", "id="+id+"&rating=all&libraries=all", owner))
	if revoked.Code != http.StatusSeeOther {
		t.Fatalf("revoke = %d", revoked.Code)
	}
	denied := serveRequest(public, requestWithCookieRequest(t, http.MethodGet, "/api/v1/library", "", session))
	if denied.Code == http.StatusOK {
		t.Fatal("revoked public browser session still works")
	}
}
