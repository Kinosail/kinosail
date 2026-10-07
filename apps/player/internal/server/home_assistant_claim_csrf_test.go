package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

const (
	claimCSRFPath  = "/api/v1/home-assistant/players/fixture-csrf"
	claimCSRFState = `{"name":"Fictional browser","state":"paused","position":1,"duration":12,"volume":0.5}`
)

type claimCSRFClient struct {
	handler http.Handler
	headers http.Header
	owner   string
	csrf    string
}

func newClaimCSRFClient(t *testing.T) *claimCSRFClient {
	t.Helper()
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://kinosail.test/settings", nil)
	request.Header.Set("User-Agent", "Kinosail browser claim regression")
	request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: owner, Secure: true, HttpOnly: true, Path: "/", SameSite: http.SameSiteStrictMode})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, request)
	csrf := regexp.MustCompile(`<meta name="kinosail-csrf" content="([^"]+)">`).FindStringSubmatch(settings.Body.String())
	if settings.Code != http.StatusOK || len(csrf) != 2 {
		t.Fatal("authenticated Settings did not supply session CSRF")
	}
	return &claimCSRFClient{handler: handler, headers: request.Header, owner: owner, csrf: csrf[1]}
}

func (client *claimCSRFClient) call(t *testing.T, method, path, body, claim string, tokens ...string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), method, "https://kinosail.test"+path, strings.NewReader(body))
	request.Header = client.headers.Clone()
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://kinosail.test")
	if claim != "" {
		request.Header.Set("X-Kinosail-Player-Claim", claim)
	}
	for _, token := range tokens {
		request.Header.Add("X-Kinosail-CSRF", token)
	}
	got := httptest.NewRecorder()
	client.handler.ServeHTTP(got, request)
	return got
}

func (client *claimCSRFClient) snapshot(t *testing.T) string {
	t.Helper()
	got := apiCall(t, client.handler, client.owner, http.MethodGet, "/api/v1/home-assistant/players", nil)
	assertAPIBody(t, got, http.StatusOK)
	return got.Body.String()
}

func (client *claimCSRFClient) rejectMutation(t *testing.T, claim, before string) {
	t.Helper()
	for _, tokens := range [][]string{nil, {"invalid"}, {client.csrf, client.csrf}} {
		for _, input := range []struct{ method, suffix, body string }{
			{http.MethodPut, "", strings.Replace(claimCSRFState, `"position":1`, `"position":9`, 1)},
			{http.MethodPost, "/release", `{}`},
		} {
			if got := client.call(t, input.method, claimCSRFPath+input.suffix, input.body, claim, tokens...); got.Code != http.StatusForbidden {
				t.Fatalf("invalid %s CSRF = %d", input.method, got.Code)
			}
			if after := client.snapshot(t); after != before {
				t.Fatal("rejected CSRF changed or released claimed player")
			}
		}
	}
}

// The actual application handler, authenticated cookie and CSRF middleware are
// exercised here. No device or household integration is created or controlled.
func TestHomeAssistantClaimReleaseAndStatePreserveBrowserCSRF(t *testing.T) {
	client := newClaimCSRFClient(t)
	const claimsPath = "/api/v1/home-assistant/players/claims"
	for _, tokens := range [][]string{nil, {"invalid"}, {client.csrf, client.csrf}} {
		if got := client.call(t, http.MethodPost, claimsPath, `{"id":"fixture-csrf"}`, "", tokens...); got.Code != http.StatusForbidden {
			t.Fatalf("invalid claim CSRF = %d", got.Code)
		}
	}
	claimed := client.call(t, http.MethodPost, claimsPath, `{"id":"fixture-csrf"}`, "", client.csrf)
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
	assertAPIBody(t, client.call(t, http.MethodPut, claimCSRFPath, claimCSRFState, ownership.Claim, client.csrf), http.StatusOK, `"command":null`)
	assertAPIBody(t, apiCall(t, client.handler, client.owner, http.MethodPost, claimCSRFPath+"/commands", map[string]any{"command": "seek", "position": 4}), http.StatusAccepted)
	client.rejectMutation(t, ownership.Claim, client.snapshot(t))
	assertAPIBody(t, client.call(t, http.MethodPut, claimCSRFPath, claimCSRFState, ownership.Claim, client.csrf), http.StatusOK, `"command":"seek"`, `"position":4`)
	assertAPIBody(t, client.call(t, http.MethodPut, claimCSRFPath, claimCSRFState, ownership.Claim, client.csrf), http.StatusOK, `"command":null`)
	if got := client.call(t, http.MethodPost, claimCSRFPath+"/release", `{}`, ownership.Claim, client.csrf); got.Code != http.StatusNoContent {
		t.Fatalf("owned protected release = %d", got.Code)
	}
	if got := client.call(t, http.MethodPost, claimsPath, `{"id":"fixture-csrf"}`, "", client.csrf); got.Code != http.StatusCreated {
		t.Fatal("rejected CSRF reserved identity or successful release did not clear it")
	}
}
