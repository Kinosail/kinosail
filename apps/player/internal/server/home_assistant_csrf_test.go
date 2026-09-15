package server_test

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

func TestHomeAssistantBrowserStateRequiresSessionCSRF(t *testing.T) {
	t.Parallel()
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://kinosail.test/settings", nil)
	request.Header.Set("User-Agent", "Kinosail browser regression")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: owner, Secure: true, HttpOnly: true, Path: "/", SameSite: http.SameSiteStrictMode})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, request)
	token := regexp.MustCompile(`<meta name="kinosail-csrf" content="([^"]+)">`).FindStringSubmatch(settings.Body.String())
	if settings.Code != http.StatusOK || len(token) != 2 {
		t.Fatal("authenticated Settings did not provide a CSRF token")
	}
	const playersPath = "/api/v1/home-assistant/players"
	before := apiCall(t, handler, owner, http.MethodGet, playersPath, nil).Body.String()
	state := `{"name":"Browser","state":"paused","title":"Track","itemId":"track","position":12,"duration":120,"volume":0.5,"muted":false}`
	for _, tokens := range [][]string{nil, {""}, {"invalid"}, {strings.Repeat("x", 4096)}, {token[1], token[1]}, {token[1]}} {
		update := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "https://kinosail.test"+playersPath+"/browser-csrf", strings.NewReader(state))
		update.Header = request.Header.Clone()
		update.Header.Set("Content-Type", "application/json")
		update.Header.Set("Origin", "https://kinosail.test")
		for _, value := range tokens {
			update.Header.Add("X-Kinosail-CSRF", value)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, update)
		if len(tokens) == 1 && tokens[0] == token[1] {
			assertAPIBody(t, response, http.StatusOK, `"command":null`)
			assertAPIBody(t, apiCall(t, handler, owner, http.MethodGet, playersPath, nil), http.StatusOK, `"id":"browser-csrf"`, `"position":12`)
			continue
		}
		if response.Code != http.StatusForbidden {
			t.Fatalf("invalid CSRF token accepted: HTTP %d", response.Code)
		}
		if after := apiCall(t, handler, owner, http.MethodGet, playersPath, nil).Body.String(); after != before {
			t.Fatal("rejected browser state changed registered players")
		}
	}
}
