package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnerFormsRejectAmbiguousFieldsWithoutSideEffects(t *testing.T) {
	handler, token := apiServer(t)
	owner := &http.Cookie{Name: "__Host-kinosail_session", Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
	t.Run("server", func(t *testing.T) { checkAmbiguousServerForms(t, handler, token, owner) })
	t.Run("timeouts", func(t *testing.T) { checkAmbiguousTimeoutForms(t, handler, token, owner) })
	t.Run("profiles", func(t *testing.T) { checkAmbiguousProfileForms(t, handler, token, owner) })
}

func checkAmbiguousServerForms(t *testing.T, handler http.Handler, token string, owner *http.Cookie) {
	t.Helper()
	for name, body := range map[string]string{
		"duplicate server name": "name=First&name=Second",
		"unknown server field":  "name=Changed&extra=true",
	} {
		t.Run(name, func(t *testing.T) {
			response := requestWithCookie(t, handler, http.MethodPost, "/settings/server", body, owner)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid server form = %d %q", response.Code, response.Body.String())
			}
			assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"name":"Kinosail"`)
		})
	}
	if response := requestWithCookie(t, handler, http.MethodPost, "/settings/server?name=QueryChanged", "", owner); response.Code != http.StatusBadRequest {
		t.Fatalf("query server form = %d %q", response.Code, response.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"name":"Kinosail"`)
	spanish := requestWithCookieRequest(t, http.MethodPost, "/settings/server", "name=First&name=Second", owner)
	spanish.AddCookie(&http.Cookie{Name: "kinosail_language", Value: "es", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	spanishResponse := httptest.NewRecorder()
	handler.ServeHTTP(spanishResponse, spanish)
	if spanishResponse.Code != http.StatusBadRequest || !strings.Contains(spanishResponse.Body.String(), "no se pudo completar la solicitud") {
		t.Fatalf("localized invalid server form = %d %q", spanishResponse.Code, spanishResponse.Body.String())
	}
}

func checkAmbiguousTimeoutForms(t *testing.T, handler http.Handler, token string, owner *http.Cookie) {
	t.Helper()
	validTimeouts := "inactiveHours=24&absoluteHours=168"
	if response := requestWithCookie(t, handler, http.MethodPost, "/settings/session-timeouts", validTimeouts, owner); response.Code != http.StatusSeeOther {
		t.Fatalf("valid timeouts = %d %q", response.Code, response.Body.String())
	}
	for name, body := range map[string]string{
		"duplicate timeout": "inactiveHours=24&inactiveHours=48&absoluteHours=168",
		"unknown timeout":   validTimeouts + "&extra=true",
	} {
		t.Run(name, func(t *testing.T) {
			response := requestWithCookie(t, handler, http.MethodPost, "/settings/session-timeouts", body, owner)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid timeout form = %d %q", response.Code, response.Body.String())
			}
			assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"sessionInactiveHours":24`, `"sessionAbsoluteHours":168`)
		})
	}
	if response := requestWithCookie(t, handler, http.MethodPost, "/settings/session-timeouts?inactiveHours=48", validTimeouts, owner); response.Code != http.StatusBadRequest {
		t.Fatalf("query timeouts form = %d %q", response.Code, response.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"sessionInactiveHours":24`, `"sessionAbsoluteHours":168`)
}

func checkAmbiguousProfileForms(t *testing.T, handler http.Handler, token string, owner *http.Cookie) {
	t.Helper()
	for name, body := range map[string]string{
		"duplicate profile": "name=Ambiguous&name=Other&password=viewer-password",
		"unknown profile":   "name=Unexpected&password=viewer-password&extra=true",
	} {
		t.Run(name, func(t *testing.T) {
			response := requestWithCookie(t, handler, http.MethodPost, "/settings/profiles", body, owner)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid profile form = %d %q", response.Code, response.Body.String())
			}
			profiles := apiCall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil)
			if profiles.Code != http.StatusOK || strings.Contains(profiles.Body.String(), `"name":"Ambiguous"`) || strings.Contains(profiles.Body.String(), `"name":"Unexpected"`) {
				t.Fatalf("invalid profile changed state = %d %q", profiles.Code, profiles.Body.String())
			}
		})
	}
	if response := requestWithCookie(t, handler, http.MethodPost, "/settings/profiles?name=QueryViewer", "password=viewer-password", owner); response.Code != http.StatusBadRequest {
		t.Fatalf("query profile form = %d %q", response.Code, response.Body.String())
	}
	profiles := apiCall(t, handler, token, http.MethodGet, "/api/v1/profiles", nil)
	if profiles.Code != http.StatusOK || strings.Contains(profiles.Body.String(), `"name":"QueryViewer"`) {
		t.Fatalf("query profile changed state = %d %q", profiles.Code, profiles.Body.String())
	}
}
