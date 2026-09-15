package servertest

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func SessionTokenSecurity(t *testing.T, handler http.Handler,
	apiCall func(*testing.T, http.Handler, string, string, string, any) *httptest.ResponseRecorder,
	mustJSON func(*testing.T, *httptest.ResponseRecorder, any),
	assertAPIBody func(*testing.T, *httptest.ResponseRecorder, int, ...string),
	testTOTP func(*testing.T, string, time.Time) string,
) {
	t.Helper()
	setup := apiCall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "totp": true})
	var owner struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	mustJSON(t, setup, &owner)
	assertAPIBody(t, apiCall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": testTOTP(t, owner.TOTP.Secret, time.Now())}), http.StatusOK)
	if response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?api_key="+owner.Token, nil); response.Code != http.StatusUnauthorized {
		t.Fatalf("API query token = %d %q", response.Code, response.Body.String())
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Users/Me?api_key="+owner.Token, nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("Jellyfin query token accepted: %d %q", response.Code, response.Body.String())
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Users/Me", nil)
	request.Header.Set("X-Emby-Token", owner.Token)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code == http.StatusUnauthorized {
		t.Fatalf("Jellyfin header token rejected: %d %q", response.Code, response.Body.String())
	}
}
