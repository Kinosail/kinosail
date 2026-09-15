package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestUpdatePreferenceAndRequestShareTheVersionedAPI(t *testing.T) {
	handler, token := apiServer(t)
	var initial struct {
		Automatic bool `json:"automatic"`
		Manager   struct {
			Status string `json:"status"`
		} `json:"manager"`
	}
	mustJSON(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/updates", nil), &initial)
	if !initial.Automatic || initial.Manager.Status != "unavailable" {
		t.Fatalf("initial updates = %#v", initial)
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, "/api/v1/settings/updates", map[string]any{"automatic": true}), http.StatusOK)

	invalid := apiCall(t, handler, token, http.MethodPost, "/api/v1/updates", map[string]any{"targetVersion": "v9.9.9"})
	assertAPIBody(t, invalid, http.StatusBadRequest, "update request must be empty")

	settings := apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil)
	assertAPIBody(t, settings, http.StatusOK, `"updates":{"automatic":true`, `"manager":{"currentVersion":`, `"status":"unavailable"`)
}

func TestOwnerCanChooseAndRequestUpdatesFromSettings(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	page := requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	assertAPIBody(t, page, http.StatusOK, `id="updates"`, "No installed update adapter has reported yet.", `name="mode" value="automatic" checked`, "Check and update now")
	for name, body := range map[string]string{"missing": "", "unknown": "mode=preview", "extra": "mode=automatic&unexpected=true"} {
		if response := requestWithCookie(t, handler, http.MethodPost, "/settings/updates", body, owner); response.Code != http.StatusBadRequest {
			t.Fatalf("%s update preference = %d %q", name, response.Code, response.Body.String())
		}
	}
	page = requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	assertAPIBody(t, page, http.StatusOK, `name="mode" value="automatic" checked`, "Check and update now")

	saved := requestWithCookie(t, handler, http.MethodPost, "/settings/updates", "mode=manual", owner)
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/settings#updates" {
		t.Fatalf("save preference = %d %q", saved.Code, saved.Header().Get("Location"))
	}
	page = requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	assertAPIBody(t, page, http.StatusOK, `name="mode" value="manual" checked`, "Approve each signed update from Settings")

	saved = requestWithCookie(t, handler, http.MethodPost, "/settings/updates", "mode=automatic", owner)
	if saved.Code != http.StatusSeeOther || saved.Header().Get("Location") != "/settings#updates" {
		t.Fatalf("re-enable automatic updates = %d %q", saved.Code, saved.Header().Get("Location"))
	}
	page = requestWithCookie(t, handler, http.MethodGet, "/settings", "", owner)
	assertAPIBody(t, page, http.StatusOK, `name="mode" value="automatic" checked`, "Install updates automatically")
}

func TestSetupUpdatePreferenceDefaultsOnAndAcceptsExplicitAutomatic(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "automaticUpdates": true, "totp": true})
	var session struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	mustJSON(t, setup, &session)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, session.TOTP.Secret, time.Now())}), http.StatusOK)
	updates := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/updates", nil)
	assertAPIBody(t, updates, http.StatusOK, `"automatic":true`)

	defaultHandler, defaultToken := apiServer(t)
	defaults := apiCall(t, defaultHandler, defaultToken, http.MethodGet, "/api/v1/updates", nil)
	assertAPIBody(t, defaults, http.StatusOK, `"automatic":true`)
}

func TestSetupRejectsUnknownUpdateModeBeforeOwnerCreation(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	page := apiCall(t, handler, "", http.MethodGet, "/setup", nil)
	assertAPIBody(t, page, http.StatusOK, `name="updateMode" value="automatic" checked`, "Turn this off if you want to approve each signed update from Settings.", `name="updateMode" value="manual"`)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader("name=Owner&password=owner-password&updateMode=preview"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid update mode = %d %q", response.Code, response.Body.String())
	}
	setup := apiCall(t, handler, "", http.MethodGet, "/setup", nil)
	assertAPIBody(t, setup, http.StatusOK, "Set up your Server.")
}
