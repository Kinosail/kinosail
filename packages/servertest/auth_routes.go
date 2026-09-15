package servertest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AuthRoutes binds common route security checks to each app's reviewed inventory and real server.
type AuthRoutes struct {
	Inventory           func(*testing.T) []string
	New                 func(*testing.T) (http.Handler, string)
	Login               func(*testing.T, http.Handler, string, string) string
	Exercise            func(*testing.T, http.Handler, string, string, bool) *httptest.ResponseRecorder
	Denied              func(*httptest.ResponseRecorder) bool
	AuthorizationDenied func(string, *httptest.ResponseRecorder) bool
	CreateProfile       func(*testing.T, http.Handler, string, map[string]any)
	JSON                func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder
	AssertPolicyDenied  func(*testing.T, *httptest.ResponseRecorder, string)
	CreateKey           func(*testing.T, http.Handler, string, string) string
	Scopes              func(string) map[string]bool
	Anonymous           map[string]bool
	Capability          map[string]bool
	OwnerOnly           map[string]bool
	FeatureGated        map[string]bool
	LocalAnonymous      map[string]bool
	AssertCapability    func(*testing.T, http.Handler, string, *httptest.ResponseRecorder)
}

func (contract AuthRoutes) AnonymousAccess(t *testing.T, handler http.Handler, routes []string) {
	for _, pattern := range routes {
		t.Run(pattern, func(t *testing.T) {
			contract.assertAnonymousRoute(t, handler, pattern)
		})
	}
}

func (contract AuthRoutes) EveryProtectedRouteAcceptsOwnerAndRejectsInvalidOrRevokedSessions(t *testing.T) {
	handler, owner := contract.New(t)
	revoked := contract.Login(t, handler, "Owner", "owner-password")
	response := contract.Exercise(t, handler, "DELETE /api/v1/session", revoked, false)
	if response.Code != http.StatusNoContent {
		t.Fatalf("revoke fixture session = %d %q", response.Code, response.Body.String())
	}

	for _, pattern := range contract.Inventory(t) {
		if contract.Anonymous[pattern] || contract.Capability[pattern] {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			contract.assertOwnerSessionStates(t, handler, pattern, owner, revoked)
		})
		if pattern == "DELETE /api/v1/session" || pattern == "POST /logout" || pattern == "POST /Sessions/Logout" {
			owner = contract.Login(t, handler, "Owner", "owner-password")
		}
	}
}

func (contract AuthRoutes) EveryProtectedRouteEnforcesOwnerAndViewerRoles(t *testing.T) {
	handler, owner := contract.New(t)
	contract.CreateProfile(t, handler, owner, map[string]any{
		"name": "Viewer", "password": "viewer-password", "rating": "all", "libraries": []string{"all"},
		"downloads": true, "transcode": true, "remote": true,
	})
	viewer := contract.Login(t, handler, "Viewer", "viewer-password")

	for _, pattern := range contract.Inventory(t) {
		if contract.Anonymous[pattern] || contract.Capability[pattern] {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			contract.assertViewerRole(t, handler, pattern, viewer)
		})
		if pattern == "DELETE /api/v1/session" || pattern == "POST /logout" || pattern == "POST /Sessions/Logout" {
			viewer = contract.Login(t, handler, "Viewer", "viewer-password")
		}
	}
}

func (contract AuthRoutes) EveryProtectedRouteEnforcesViewerSchedule(t *testing.T) {
	handler, owner := contract.New(t)
	created := contract.JSON(t, handler, owner, http.MethodPost, "/api/v1/profiles", map[string]any{
		"name": "Scheduled Viewer", "password": "scheduled-password", "rating": "all", "libraries": []string{"all"},
		"downloads": true, "transcode": true, "remote": true,
	})
	var profile struct {
		ID string `json:"id"`
	}
	if created.Code != http.StatusCreated || json.Unmarshal(created.Body.Bytes(), &profile) != nil || profile.ID == "" {
		t.Fatalf("create scheduled Viewer = %d %q", created.Code, created.Body.String())
	}
	scheduled := contract.Login(t, handler, "Scheduled Viewer", "scheduled-password")
	updated := contract.JSON(t, handler, owner, http.MethodPut, "/api/v1/profiles/"+profile.ID, map[string]any{"accessStart": "00:00", "accessEnd": "00:00"})
	if updated.Code != http.StatusOK {
		t.Fatalf("schedule Viewer = %d %q", updated.Code, updated.Body.String())
	}

	for _, pattern := range contract.Inventory(t) {
		if contract.Anonymous[pattern] || contract.Capability[pattern] {
			continue
		}
		t.Run(pattern, func(t *testing.T) {
			outsideHours := contract.Exercise(t, handler, pattern, scheduled, false)
			contract.AssertPolicyDenied(t, outsideHours, "viewing schedule")
		})
	}
}

func (contract AuthRoutes) EveryProtectedRouteEnforcesExactAPIKeyScopes(t *testing.T) {
	handler, owner := contract.New(t)
	keys := make(map[string]string)
	for _, scope := range []string{"library", "write", "stream", "download", "admin"} {
		keys[scope] = contract.CreateKey(t, handler, owner, scope)
	}

	for _, pattern := range contract.Inventory(t) {
		if contract.Anonymous[pattern] || contract.Capability[pattern] {
			continue
		}
		expected := contract.Scopes(pattern)
		t.Run(pattern, func(t *testing.T) {
			for scope, key := range keys {
				response := contract.Exercise(t, handler, pattern, key, false)
				contract.assertKeyScope(t, pattern, scope, expected[scope], response)
			}
		})
	}
}

func (contract AuthRoutes) assertAnonymousRoute(t *testing.T, handler http.Handler, pattern string) {
	t.Helper()
	response := contract.Exercise(t, handler, pattern, "", false)
	if contract.FeatureGated[pattern] {
		if response.Code != http.StatusNotFound {
			t.Fatalf("disabled Home Assistant route = %d %q", response.Code, response.Body.String())
		}
		return
	}
	if contract.Anonymous[pattern] {
		if contract.Denied(response) {
			t.Fatalf("explicit anonymous route was blocked: %d %q", response.Code, response.Body.String())
		}
		return
	}
	if contract.LocalAnonymous[pattern] {
		if contract.Denied(response) {
			t.Fatalf("local Jellyfin compatibility route was blocked: %d %q", response.Code, response.Body.String())
		}
		return
	}
	if contract.Capability[pattern] {
		contract.AssertCapability(t, handler, pattern, response)
		return
	}
	if !contract.Denied(response) {
		t.Fatalf("protected route was reachable anonymously: %d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
}

func (contract AuthRoutes) assertKeyScope(t *testing.T, pattern, scope string, allowed bool, response *httptest.ResponseRecorder) {
	t.Helper()
	if allowed {
		if contract.AuthorizationDenied(pattern, response) {
			t.Errorf("%s scope was denied: %d location=%q body=%q", scope, response.Code, response.Header().Get("Location"), response.Body.String())
		}
		return
	}
	if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), "API key scope does not allow this request") {
		t.Errorf("%s scope reached route: %d location=%q body=%q", scope, response.Code, response.Header().Get("Location"), response.Body.String())
	}
}

// ReviewedAnonymousAccess verifies the exact inventory before exercising every route.
func (contract AuthRoutes) ReviewedAnonymousAccess(t *testing.T, expectedCount int, expectedDigest string, newAnonymous func(*testing.T, string) http.Handler) {
	routes := contract.Inventory(t)
	if len(routes) != expectedCount {
		t.Fatalf("registered route count = %d, want %d", len(routes), expectedCount)
	}
	digest := sha256.Sum256([]byte(strings.Join(routes, "\n") + "\n"))
	if actual := hex.EncodeToString(digest[:]); actual != expectedDigest {
		t.Fatalf("registered route inventory changed: sha256 = %s, want %s; review and classify every changed route", actual, expectedDigest)
	}

	data := t.TempDir()
	if err := os.WriteFile(filepath.Join(data, "settings.json"), []byte(`{"name":"Kinosail","libraries":["."],"jellyfinCompatibility":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := newAnonymous(t, data)

	contract.AnonymousAccess(t, handler, routes)
}

func (contract AuthRoutes) assertOwnerSessionStates(t *testing.T, handler http.Handler, pattern, owner, revoked string) {
	t.Helper()
	accepted := contract.Exercise(t, handler, pattern, owner, false)
	if contract.AuthorizationDenied(pattern, accepted) {
		t.Fatalf("valid Owner was denied: %d location=%q body=%q", accepted.Code, accepted.Header().Get("Location"), accepted.Body.String())
	}

	invalid := contract.Exercise(t, handler, pattern, "not-a-valid-session", false)
	if !contract.Denied(invalid) {
		t.Fatalf("malformed credential was accepted: %d location=%q body=%q", invalid.Code, invalid.Header().Get("Location"), invalid.Body.String())
	}

	expired := contract.Exercise(t, handler, pattern, revoked, false)
	if !contract.Denied(expired) {
		t.Fatalf("revoked credential was accepted: %d location=%q body=%q", expired.Code, expired.Header().Get("Location"), expired.Body.String())
	}
}

func (contract AuthRoutes) assertViewerRole(t *testing.T, handler http.Handler, pattern, viewer string) {
	t.Helper()
	response := contract.Exercise(t, handler, pattern, viewer, false)
	if contract.OwnerOnly[pattern] {
		if !strings.Contains(response.Body.String(), "Owner access required") || response.Code != http.StatusForbidden {
			t.Fatalf("Viewer reached Owner route: %d %q", response.Code, response.Body.String())
		}
		return
	}
	if contract.AuthorizationDenied(pattern, response) {
		t.Fatalf("authorized Viewer was denied: %d location=%q body=%q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
}
