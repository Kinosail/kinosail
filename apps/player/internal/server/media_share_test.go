package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOwnerCreatesClaimsStreamsAndRevokesMediaShare(t *testing.T) { //nolint:cyclop // One public-interface lifecycle proves scope, claiming, delivery, and revocation.
	handler, owner := apiServer(t)
	library := apiCall(t, handler, owner, http.MethodGet, "/api/v1/library", nil)
	var items struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, library, &items)
	created := apiCall(t, handler, owner, http.MethodPost, "/api/v1/media-shares", map[string]any{"itemIds": []string{items.Items[0].ID}, "expiresInSeconds": 3600, "maxDevices": 1, "rightsAcknowledged": true})
	var share struct{ ID, ClaimToken string }
	mustJSON(t, created, &share)
	if created.Code != http.StatusCreated || share.ID == "" || len(share.ClaimToken) < 32 || strings.Contains(created.Body.String(), "playSessionId") {
		t.Fatalf("created share = %d %q", created.Code, created.Body.String())
	}
	claimed := apiCall(t, handler, "", http.MethodPost, "/api/v1/media-shares/claim", map[string]any{"token": share.ClaimToken, "device": "Guest browser"})
	if claimed.Code != http.StatusNoContent || len(claimed.Result().Cookies()) != 1 || claimed.Result().Cookies()[0].Name != "__Host-kinosail_share" || !claimed.Result().Cookies()[0].HttpOnly {
		t.Fatalf("claim = %d cookies=%#v body=%q", claimed.Code, claimed.Result().Cookies(), claimed.Body.String())
	}
	media := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share/media/"+items.Items[0].ID, nil)
	media.AddCookie(claimed.Result().Cookies()[0])
	streamed := httptest.NewRecorder()
	handler.ServeHTTP(streamed, media)
	if streamed.Code != http.StatusOK || streamed.Body.String() != "video" {
		t.Fatalf("stream = %d %q", streamed.Code, streamed.Body.String())
	}
	if response := apiCall(t, handler, owner, http.MethodDelete, "/api/v1/media-shares/"+share.ID, nil); response.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d %q", response.Code, response.Body.String())
	}
	denied := httptest.NewRecorder()
	handler.ServeHTTP(denied, media.Clone(t.Context()))
	if denied.Code != http.StatusNotFound {
		t.Fatalf("revoked stream = %d %q", denied.Code, denied.Body.String())
	}
}

func TestAuthenticatedMediaShareClaimCarriesCSRFToken(t *testing.T) {
	handler, owner := apiServer(t)
	library := apiCall(t, handler, owner, http.MethodGet, "/api/v1/library", nil)
	var items struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	mustJSON(t, library, &items)
	created := apiCall(t, handler, owner, http.MethodPost, "/api/v1/media-shares", map[string]any{"itemIds": []string{items.Items[0].ID}, "expiresInSeconds": 3600, "maxDevices": 1, "rightsAcknowledged": true})
	var share struct{ ClaimToken string }
	mustJSON(t, created, &share)

	cookie := &http.Cookie{Name: "__Host-kinosail_session", Value: "browser-session", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode}
	pageRequest := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/share", nil)
	pageRequest.Header.Set("User-Agent", "Mozilla/5.0")
	pageRequest.AddCookie(cookie)
	pageResponse := httptest.NewRecorder()
	handler.ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("claim page = %d %q", pageResponse.Code, pageResponse.Body.String())
	}
	const csrfMarker = `<meta name="kinosail-csrf" content="`
	start := strings.Index(pageResponse.Body.String(), csrfMarker)
	if start < 0 {
		t.Fatalf("claim page lacks CSRF meta: %q", pageResponse.Body.String())
	}
	csrf := pageResponse.Body.String()[start+len(csrfMarker):]
	csrf = csrf[:strings.IndexByte(csrf, '"')]
	if csrf == "" {
		t.Fatal("claim page has an empty CSRF token")
	}

	script := apiCall(t, handler, "", http.MethodGet, "/static/media-share.js", nil)
	if script.Code != http.StatusOK || !strings.Contains(script.Body.String(), `meta[name="kinosail-csrf"]`) || !strings.Contains(script.Body.String(), `"X-Kinosail-CSRF"`) {
		t.Fatalf("claim script lacks CSRF header support: %d %q", script.Code, script.Body.String())
	}

	claim := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/media-shares/claim", strings.NewReader(`{"token":"`+share.ClaimToken+`","device":"Authenticated browser"}`))
	claim.Header.Set("Content-Type", "application/json")
	claim.Header.Set("Origin", "http://example.com")
	claim.Header.Set("User-Agent", "Mozilla/5.0")
	claim.AddCookie(cookie)
	withoutToken := httptest.NewRecorder()
	handler.ServeHTTP(withoutToken, claim.Clone(t.Context()))
	if withoutToken.Code != http.StatusForbidden || !strings.Contains(withoutToken.Body.String(), "cross-origin request denied") {
		t.Fatalf("claim without CSRF = %d %q", withoutToken.Code, withoutToken.Body.String())
	}
	claim.Header.Set("X-Kinosail-CSRF", csrf)
	withToken := httptest.NewRecorder()
	handler.ServeHTTP(withToken, claim)
	if withToken.Code != http.StatusNoContent {
		t.Fatalf("claim with CSRF = %d %q", withToken.Code, withToken.Body.String())
	}
}

func TestMediaShareRejectsUnknownOversizedAndAmbiguousInputWithoutGrant(t *testing.T) {
	handler, owner := apiServer(t)
	for name, body := range map[string]any{
		"missing rights":   map[string]any{"itemIds": []string{"missing"}, "expiresInSeconds": 3600, "maxDevices": 1},
		"unknown item":     map[string]any{"itemIds": []string{"missing"}, "expiresInSeconds": 3600, "maxDevices": 1, "rightsAcknowledged": true},
		"too long":         map[string]any{"itemIds": []string{"missing"}, "expiresInSeconds": 86401, "maxDevices": 1, "rightsAcknowledged": true},
		"too many devices": map[string]any{"itemIds": []string{"missing"}, "expiresInSeconds": 3600, "maxDevices": 9, "rightsAcknowledged": true},
		"unknown field":    json.RawMessage(`{"itemIds":["missing"],"expiresInSeconds":3600,"maxDevices":1,"rightsAcknowledged":true,"owner":true}`),
	} {
		t.Run(name, func(t *testing.T) {
			response := apiCall(t, handler, owner, http.MethodPost, "/api/v1/media-shares", body)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid share = %d %q", response.Code, response.Body.String())
			}
		})
	}
	list := apiCall(t, handler, owner, http.MethodGet, "/api/v1/media-shares", nil)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), `"shares":[]`) {
		t.Fatalf("rejected input created a share: %d %q", list.Code, list.Body.String())
	}
}
