package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHTTPAuthenticationAndCSRF(t *testing.T) {
	app := newTestApplication(t, Config{SecureCookies: true})

	unauthorized := app.request(t, http.MethodGet, "/api/v1/board", "", nil)
	requireJSONError(t, unauthorized, http.StatusUnauthorized, "authentication required")

	session := app.setup(t)
	setupResponse := app.request(t, http.MethodPost, "/api/v1/session", `{"name":"Owner","password":"long-password-123","device":"second browser"}`, nil)
	if setupResponse.Header().Get("X-Kinosail-Login-Next") != "/?passkey=offer" {
		t.Fatalf("password login next = %q", setupResponse.Header().Get("X-Kinosail-Login-Next"))
	}
	if !session.cookie.HttpOnly || !session.cookie.Secure || session.cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("unsafe session cookie: %#v", session.cookie)
	}

	missingCSRF := app.request(t, http.MethodPost, "/api/v1/apps", `{"name":"Router","url":"http://router.lan","expectedVersion":1}`, &testSession{cookie: session.cookie})
	requireJSONError(t, missingCSRF, http.StatusForbidden, "request verification failed")
	if got := len(app.board.Snapshot().Apps); got != 0 {
		t.Fatalf("missing CSRF created %d applications", got)
	}

	bearerRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/apps", strings.NewReader(`{"name":"Router","url":"http://router.lan","expectedVersion":1}`))
	bearerRequest.Header.Set("Content-Type", "application/json")
	bearerRequest.Header.Set("Authorization", "Bearer "+session.cookie.Value)
	bearerResponse := httptest.NewRecorder()
	app.handler.ServeHTTP(bearerResponse, bearerRequest)
	if bearerResponse.Code != http.StatusCreated {
		t.Fatalf("bearer mutation status = %d, body = %s", bearerResponse.Code, bearerResponse.Body.String())
	}

	rename := app.request(t, http.MethodPut, "/api/v1/board", `{"title":"Household","expectedVersion":2}`, &session)
	if rename.Code != http.StatusOK {
		t.Fatalf("cookie mutation status = %d, body = %s", rename.Code, rename.Body.String())
	}

	assertLogoutRequiresCSRFAndRevokesSession(t, app, session)
}

func assertLogoutRequiresCSRFAndRevokesSession(t *testing.T, app testApplication, session testSession) {
	t.Helper()
	withoutLogoutCSRF := app.request(t, http.MethodDelete, "/api/v1/session", `{}`, &testSession{cookie: session.cookie})
	requireJSONError(t, withoutLogoutCSRF, http.StatusForbidden, "request verification failed")
	logout := app.request(t, http.MethodDelete, "/api/v1/session", `{}`, &session)
	if logout.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, body = %s", logout.Code, logout.Body.String())
	}
	logoutCookies := logout.Result().Cookies()
	if len(logoutCookies) != 1 || logoutCookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("logout cookies = %#v, want one SameSite Strict cookie", logoutCookies)
	}
	afterLogout := app.request(t, http.MethodGet, "/api/v1/board", "", &session)
	requireJSONError(t, afterLogout, http.StatusUnauthorized, "authentication required")
}

func TestMalformedAuthorizationDoesNotFallBackToValidCookie(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	for name, authorization := range map[string]string{
		"wrong scheme":  "Basic credentials",
		"missing token": "Bearer",
		"extra value":   "Bearer token extra",
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/board", nil)
			request.AddCookie(session.cookie)
			request.Header.Set("Authorization", authorization)
			response := httptest.NewRecorder()
			app.handler.ServeHTTP(response, request)
			requireJSONError(t, response, http.StatusUnauthorized, "authentication required")
		})
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/apps", strings.NewReader(`{"name":"Router","url":"http://router.lan","expectedVersion":1}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Basic credentials")
	request.Header.Set("X-Kinosail-CSRF", session.csrf)
	request.AddCookie(session.cookie)
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	requireJSONError(t, response, http.StatusUnauthorized, "authentication required")
	if got := len(app.board.Snapshot().Apps); got != 0 {
		t.Fatalf("malformed Authorization created %d applications", got)
	}
}

func TestSetupRejectsUnknownOrMisCasedJSONWithoutConfiguringOwner(t *testing.T) {
	tests := map[string]string{
		"unknown":   `{"name":"Owner","password":"long-password-123","device":"browser","admin":true}`,
		"mis-cased": `{"Name":"Owner","password":"long-password-123","device":"browser"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			app := newTestApplication(t, Config{})
			response := app.request(t, http.MethodPost, "/api/v1/setup", body, nil)
			requireJSONError(t, response, http.StatusBadRequest, "invalid JSON request")
			if app.auth.Configured() {
				t.Fatal("invalid setup configured the Owner")
			}
		})
	}
}

func TestLoginRateLimit(t *testing.T) {
	app := newTestApplication(t, Config{})
	app.setup(t)
	body := `{"name":"Owner","password":"` + strings.Repeat("x", 73) + `","device":"test browser"}`
	for attempt := 1; attempt <= perSourceLogins; attempt++ {
		response := app.request(t, http.MethodPost, "/api/v1/session", body, nil)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d, body = %s", attempt, response.Code, response.Body.String())
		}
	}
	response := app.request(t, http.MethodPost, "/api/v1/session", body, nil)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("limited status = %d, body = %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Retry-After") == "" {
		t.Fatal("rate-limited response omitted Retry-After")
	}
}

func TestCrossOriginMutationIsRejectedWithoutSideEffects(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	tests := map[string]map[string]string{
		"fetch metadata": {"Sec-Fetch-Site": "cross-site"},
		"origin":         {"Origin": "https://outside.example"},
	}
	for name, headers := range tests {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/apps", bytes.NewBufferString(`{"name":"Router","url":"http://router.lan","expectedVersion":1}`))
			request.Header.Set("Content-Type", "application/json")
			for header, value := range headers {
				request.Header.Set(header, value)
			}
			request.AddCookie(session.cookie)
			request.Header.Set("X-Kinosail-CSRF", session.csrf)
			response := httptest.NewRecorder()
			app.handler.ServeHTTP(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("cross-origin status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
	if got := len(app.board.Snapshot().Apps); got != 0 {
		t.Fatalf("cross-origin request created %d applications", got)
	}
}

func TestUntrustedHostIsRejectedBeforeRouting(t *testing.T) {
	app := newTestApplication(t, Config{})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil)
	request.Host = "outside.example"
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("untrusted host status = %d, body = %s", response.Code, response.Body.String())
	}
	if body := response.Body.String(); !strings.Contains(body, "request host is not allowed") {
		t.Fatalf("untrusted host body = %q", body)
	}
}
