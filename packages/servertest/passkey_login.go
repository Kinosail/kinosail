package servertest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// PasskeyLoginIsAvailableFromSignInPage verifies the safe return target and passkey-first login actions.
func PasskeyLoginIsAvailableFromSignInPage(t *testing.T, handler http.Handler, scriptPath string) {
	_ = SetupOwnerCookie(t, handler)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login?stepup=1&next=%2Fsettings%2Fbackups", nil))
	page := response.Body.String()
	if response.Code != http.StatusOK || !strings.Contains(page, scriptPath) || !strings.Contains(page, `data-passkey-login`) || !strings.Contains(page, `data-login-next="/settings/backups"`) || !strings.Contains(page, `data-passkey-status`) {
		t.Fatalf("login page = %d %q", response.Code, page)
	}
	if passkey, password := strings.Index(page, `data-passkey-login`), strings.Index(page, `<button class="quiet">Sign in</button>`); passkey < 0 || password < 0 || passkey > password {
		t.Fatalf("sign-in actions are not passkey-first: %q", page)
	}
}

// PasskeyLoginBeginIsPublic verifies the unauthenticated passkey discovery response.
func PasskeyLoginBeginIsPublic(t *testing.T, handler http.Handler, origin string) {
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/passkeys/login/begin", nil)
	request.Host = strings.TrimPrefix(origin, "https://")
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"rpId":"localhost"`) || !strings.Contains(response.Body.String(), `"userVerification":"required"`) {
		t.Fatalf("login begin = %d %q", response.Code, response.Body.String())
	}
}

// PasskeyLoginIgnoresStaleSession verifies public ceremonies do not reject stale cookies.
func PasskeyLoginIgnoresStaleSession(t *testing.T, handler http.Handler, origin string) {
	for path, want := range map[string]int{
		"/api/v1/passkeys/login/begin":  http.StatusOK,
		"/api/v1/passkeys/login/finish": http.StatusBadRequest,
		"/auth/passkeys/login/begin":    http.StatusOK,
		"/auth/passkeys/login/finish":   http.StatusBadRequest,
	} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+path, strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", origin)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		request.Header.Set("User-Agent", "Mozilla/5.0")
		request.AddCookie(&http.Cookie{Name: "__Host-kinosail_session", Value: "stale-session", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != want || strings.Contains(response.Body.String(), "cross-origin request denied") {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
}

// PasskeyBeginRedirectsToConfiguredOrigin verifies an alternate trusted host cannot mint ceremony state.
func PasskeyBeginRedirectsToConfiguredOrigin(t *testing.T, handler http.Handler, authOrigin, requestOrigin string) {
	setup := httptest.NewRequestWithContext(t.Context(), http.MethodPost, authOrigin+"/setup", strings.NewReader("name=Owner&password=owner-password"))
	setup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	created := httptest.NewRecorder()
	handler.ServeHTTP(created, setup)
	owner := created.Result().Cookies()[0]
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, requestOrigin+"/api/v1/passkeys/register/begin", nil)
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusMisdirectedRequest || response.Header().Get("Location") != authOrigin+"/account" || len(response.Result().Cookies()) != 0 {
		t.Fatalf("passkey origin = %d, location = %q, body = %q", response.Code, response.Header().Get("Location"), response.Body.String())
	}
}
