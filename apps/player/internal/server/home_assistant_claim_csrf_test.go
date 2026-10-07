package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The actual application handler, authenticated cookie and CSRF middleware are
// exercised here. No device or household integration is created or controlled.
func TestHomeAssistantClaimReleaseAndStatePreserveBrowserCSRF(t *testing.T) {
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK)
	settingsRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://kinosail.test/settings", nil)
	settingsRequest.Header.Set("User-Agent", "Kinosail browser claim regression")
	settingsRequest.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: owner, Secure: true, HttpOnly: true, Path: "/", SameSite: http.SameSiteStrictMode})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, settingsRequest)
	csrf := regexp.MustCompile(`<meta name="kinosail-csrf" content="([^"]+)">`).FindStringSubmatch(settings.Body.String())
	if settings.Code != http.StatusOK || len(csrf) != 2 {
		t.Fatal("authenticated Settings did not supply session CSRF")
	}
	call := func(method, path, body, claim string, tokens ...string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), method, "https://kinosail.test"+path, strings.NewReader(body))
		req.Header = settingsRequest.Header.Clone()
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://kinosail.test")
		if claim != "" {
			req.Header.Set("X-Kinosail-Player-Claim", claim)
		}
		for _, token := range tokens {
			req.Header.Add("X-Kinosail-CSRF", token)
		}
		got := httptest.NewRecorder()
		handler.ServeHTTP(got, req)
		return got
	}
	const claimsPath = "/api/v1/home-assistant/players/claims"
	for _, tokens := range [][]string{nil, {"invalid"}, {csrf[1], csrf[1]}} {
		if got := call(http.MethodPost, claimsPath, `{"id":"fixture-csrf"}`, "", tokens...); got.Code != http.StatusForbidden {
			t.Fatalf("invalid claim CSRF = %d", got.Code)
		}
	}
	claimed := call(http.MethodPost, claimsPath, `{"id":"fixture-csrf"}`, "", csrf[1])
	if claimed.Code != http.StatusCreated {
		t.Fatalf("protected claim prerequisite = %d", claimed.Code)
	}
	var ownership struct {
		ID    string `json:"id"`
		Claim string `json:"claim"`
	}
	if json.Unmarshal(claimed.Body.Bytes(), &ownership) != nil || ownership.ID != "fixture-csrf" || len(ownership.Claim) < 20 {
		t.Fatal("protected claim response invalid")
	}
	const state = `{"name":"Fictional browser","state":"paused","position":1,"duration":12,"volume":0.5}`
	path := "/api/v1/home-assistant/players/fixture-csrf"
	assertAPIBody(t, call(http.MethodPut, path, state, ownership.Claim, csrf[1]), http.StatusOK, `"command":null`)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPost, path+"/commands", map[string]any{"command": "seek", "position": 4}), http.StatusAccepted)
	before := apiCall(t, handler, owner, http.MethodGet, "/api/v1/home-assistant/players", nil).Body.String()
	for _, tokens := range [][]string{nil, {"invalid"}, {csrf[1], csrf[1]}} {
		for _, method := range []string{http.MethodPut, http.MethodPost} {
			target, body := path, strings.Replace(state, `"position":1`, `"position":9`, 1)
			if method == http.MethodPost {
				target, body = path+"/release", `{}`
			}
			if got := call(method, target, body, ownership.Claim, tokens...); got.Code != http.StatusForbidden {
				t.Fatalf("invalid %s CSRF = %d", method, got.Code)
			}
			if after := apiCall(t, handler, owner, http.MethodGet, "/api/v1/home-assistant/players", nil).Body.String(); after != before {
				t.Fatal("rejected CSRF changed or released claimed player")
			}
		}
	}
	assertAPIBody(t, call(http.MethodPut, path, state, ownership.Claim, csrf[1]), http.StatusOK, `"command":"seek"`, `"position":4`)
	assertAPIBody(t, call(http.MethodPut, path, state, ownership.Claim, csrf[1]), http.StatusOK, `"command":null`)
	if got := call(http.MethodPost, path+"/release", `{}`, ownership.Claim, csrf[1]); got.Code != http.StatusNoContent {
		t.Fatalf("owned protected release = %d", got.Code)
	}
	if got := call(http.MethodPost, claimsPath, `{"id":"fixture-csrf"}`, "", csrf[1]); got.Code != http.StatusCreated {
		t.Fatal("rejected CSRF reserved identity or successful release did not clear it")
	}
}
