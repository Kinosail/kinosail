package server

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func TestManagementPageIsPrivateAndDefaultsOff(t *testing.T) {
	handler, owner := newRouteAuthorizationServer(t)
	page := exerciseRoute(t, handler, "GET /settings/management", owner, false)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), "Off · manage at home") || !strings.Contains(page.Body.String(), "separate setting") {
		t.Fatalf("management page = %d %s", page.Code, page.Body.String())
	}
	status := exerciseRoute(t, handler, "GET /api/v1/management-access", owner, false)
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"enabled":false`) {
		t.Fatalf("management status = %d", status.Code)
	}
	createRouteProfile(t, handler, owner, map[string]any{"name": "Viewer", "password": "viewer-password"})
	viewer := loginRouteProfile(t, handler, "Viewer", "viewer-password")
	for _, route := range []string{"POST /settings/management/enable", "POST /settings/management/disable", "POST /settings/management/devices", "POST /settings/management/devices/revoke", "GET /settings/management", "GET /api/v1/management-access", "POST /api/v1/management-access", "DELETE /api/v1/management-access", "POST /api/v1/management-access/devices", "DELETE /api/v1/management-access/devices"} {
		denied := exerciseRoute(t, handler, route, viewer, false)
		if !routeAuthorizationDenied(route, denied) {
			t.Fatalf("Viewer reached %s: %d", route, denied.Code)
		}
		public := exerciseRoute(t, identitycore.Remote(handler), route, owner, false)
		if public.Code != http.StatusNotFound {
			t.Fatalf("public management route %s = %d", route, public.Code)
		}
	}
}

func TestManagementSessionReplayCannotTouchAppSessionState(t *testing.T) {
	store := newProfileStore(t.TempDir())
	// Resolve through the real app token boundary; it must reject before LastSeen or persistence.
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	store.sessions[sessionKey("management-token")] = viewerSession{ProfileID: "owner", ManagementDevice: key, Browser: true, LastSeen: 1, ExpiresAt: 9999999999}
	before, err := json.Marshal(store.sessions)
	if err != nil {
		t.Fatal(err)
	}
	writes := 0
	store.persist = func(string, any) error { writes++; return nil }
	for _, bound := range []bool{false, true} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		request.Header.Set("Authorization", "Bearer management-token")
		if bound {
			request = identitycore.WithManagementDevice(request, "owner", base64.StdEncoding.EncodeToString([]byte(strings.Repeat("q", 32))))
		}
		if _, ok := store.profile(request); ok {
			t.Fatal("replayed Owner session authenticated")
		}
	}
	after, err := json.Marshal(store.sessions)
	if err != nil || string(before) != string(after) || writes != 0 {
		t.Fatal("replayed session changed app state")
	}
}

func TestManagementIntegrationsCannotBypassOwnerSession(t *testing.T) {
	handler, _ := newRouteAuthorizationServer(t)
	key := base64.StdEncoding.EncodeToString([]byte(strings.Repeat("k", 32)))
	for _, path := range []string{"/scim/v2/Users", "/oauth/register", "/oauth/token", "/api/v1/setup", "/api/v1/home-assistant/pair"} {
		r := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer integration-token")
		r = identitycore.WithManagementDevice(r, "owner", key)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		if w.Code != http.StatusUnauthorized && w.Code != http.StatusSeeOther {
			t.Fatalf("integration bypassed Owner authentication: %s = %d", path, w.Code)
		}
	}
}
