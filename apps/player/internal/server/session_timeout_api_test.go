package server_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestOwnerCanCustomizeSessionTimeoutsThroughAPIAndWeb(t *testing.T) {
	handler, token := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"sessionInactiveHours":0.25`, `"sessionAbsoluteHours":8`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, "/api/v1/settings/session-timeouts", map[string]any{"inactiveHours": .25, "absoluteHours": 8760}), http.StatusOK, `"status":"saved"`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"sessionInactiveHours":0.25`, `"sessionAbsoluteHours":8760`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPut, "/api/v1/settings/session-timeouts", map[string]any{"inactiveHours": 24, "absoluteHours": 168}), http.StatusOK, `"status":"saved"`)
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"sessionInactiveHours":24`, `"sessionAbsoluteHours":168`)

	page := apiCall(t, handler, token, http.MethodGet, "/settings", nil)
	assertAPIBody(t, page, http.StatusOK, `action="/settings/session-timeouts"`, `15 minutes (recommended)`, `8 hours (recommended)`, `value="8760"`, `value="24" selected`, `value="168" selected`)

	invalid := requestWithCookie(t, handler, http.MethodPost, "/settings/session-timeouts", url.Values{"inactiveHours": {"48"}, "absoluteHours": {"24"}}.Encode(), &http.Cookie{Name: "__Host-kinosail_session", Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	if invalid.Code != http.StatusBadRequest || !strings.Contains(invalid.Body.String(), "session timeouts are invalid") {
		t.Fatalf("invalid web timeouts = %d %q", invalid.Code, invalid.Body.String())
	}
	invalid = requestWithCookie(t, handler, http.MethodPost, "/settings/session-timeouts", url.Values{"inactiveHours": {"NaN"}, "absoluteHours": {"168"}}.Encode(), &http.Cookie{Name: "__Host-kinosail_session", Value: token, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("non-finite web timeout = %d", invalid.Code)
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"sessionInactiveHours":24`, `"sessionAbsoluteHours":168`)
}
