package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestUnsafePasswordsAreRejectedWithoutAccountSideEffects(t *testing.T) {
	handler := server.New(server.Config{DataDir: t.TempDir(), RequireAuth: true})
	rejected := map[string]string{"common": "qwertyqwerty", "oversized": strings.Repeat("x", 1025)}
	for name, password := range rejected {
		assertRejectedPasswordSetup(t, handler, name, password)
	}
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var owner struct {
		Token string
		TOTP  struct{ Secret string }
	}
	mustJSON(t, setup, &owner)
	if setup.Code != http.StatusCreated {
		t.Fatalf("valid setup after rejected password = %d %q", setup.Code, setup.Body.String())
	}
	if response := apiCall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, owner.TOTP.Secret, time.Now())}); response.Code != http.StatusOK {
		t.Fatalf("enable owner MFA = %d %q", response.Code, response.Body.String())
	}
	for name, password := range rejected {
		if response := apiCall(t, handler, owner.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": password}); response.Code != http.StatusBadRequest {
			t.Fatalf("%s-password profile create = %d %q", name, response.Code, response.Body.String())
		}
	}
	created := apiCall(t, handler, owner.Token, http.MethodPost, "/api/v1/profiles", map[string]any{"name": "Viewer", "password": "viewer-password"})
	var viewer struct{ ID string }
	mustJSON(t, created, &viewer)
	if created.Code != http.StatusCreated {
		t.Fatalf("create Viewer = %d %q", created.Code, created.Body.String())
	}
	for name, password := range rejected {
		if response := apiCall(t, handler, owner.Token, http.MethodPut, "/api/v1/profiles/"+viewer.ID+"/password", map[string]any{"password": password}); response.Code != http.StatusBadRequest {
			t.Fatalf("%s-password reset = %d %q", name, response.Code, response.Body.String())
		}
	}
	if response := apiCall(t, handler, "", http.MethodPost, "/api/v1/session", map[string]any{"name": "Viewer", "password": "viewer-password"}); response.Code != http.StatusCreated {
		t.Fatalf("rejected reset changed password = %d %q", response.Code, response.Body.String())
	}
}

func assertRejectedPasswordSetup(t *testing.T, handler http.Handler, name, password string) {
	t.Helper()
	webSetup := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/setup", strings.NewReader("name=Owner&password="+password))
	webSetup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	webResponse := httptest.NewRecorder()
	handler.ServeHTTP(webResponse, webSetup)
	if webResponse.Code != http.StatusBadRequest {
		t.Fatalf("%s-password web setup = %d %q", name, webResponse.Code, webResponse.Body.String())
	}
	if body := webResponse.Body.String(); !strings.Contains(body, "Check your setup details.") || !strings.Contains(body, "Try again") || !strings.Contains(body, "Nothing was saved") {
		t.Fatalf("%s-password web setup error is not recoverable: %q", name, body)
	}
}
