package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPasskeyRoutesEnforceOriginAuthenticationAndBoundedCeremonies(t *testing.T) {
	app := newTestApplication(t, Config{PublicURL: "https://dashboard.example:38400", SecureCookies: true, TrustedHosts: []string{"dashboard.example"}})
	session := app.setup(t)

	mismatched := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://example.com/api/v1/passkeys/login/begin", nil)
	mismatchResponse := httptest.NewRecorder()
	app.handler.ServeHTTP(mismatchResponse, mismatched)
	requireJSONError(t, mismatchResponse, http.StatusMisdirectedRequest, "use the configured Dashboard address for passkeys")
	if mismatchResponse.Header().Get("Location") != "https://dashboard.example:38400/login" {
		t.Fatalf("passkey redirect = %q", mismatchResponse.Header().Get("Location"))
	}

	unauthorized := passkeyRequest(t, "/api/v1/passkeys/register/begin", "", nil)
	unauthorizedResponse := httptest.NewRecorder()
	app.handler.ServeHTTP(unauthorizedResponse, unauthorized)
	requireJSONError(t, unauthorizedResponse, http.StatusUnauthorized, "authentication required")

	missingCSRF := passkeyRequest(t, "/api/v1/passkeys/register/begin", "", &session)
	missingCSRF.Header.Del("X-Kinosail-CSRF")
	missingCSRFResponse := httptest.NewRecorder()
	app.handler.ServeHTTP(missingCSRFResponse, missingCSRF)
	requireJSONError(t, missingCSRFResponse, http.StatusForbidden, "request verification failed")

	unexpectedBody := passkeyRequest(t, "/api/v1/passkeys/register/begin", `{}`, &session)
	unexpectedBodyResponse := httptest.NewRecorder()
	app.handler.ServeHTTP(unexpectedBodyResponse, unexpectedBody)
	requireJSONError(t, unexpectedBodyResponse, http.StatusBadRequest, "request body must be empty")

	begin := passkeyRequest(t, "/api/v1/passkeys/register/begin", "", &session)
	beginResponse := httptest.NewRecorder()
	app.handler.ServeHTTP(beginResponse, begin)
	if beginResponse.Code != http.StatusOK {
		t.Fatalf("begin status = %d, body = %s", beginResponse.Code, beginResponse.Body.String())
	}
	cookies := beginResponse.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].Path != "/api/v1/passkeys/" {
		t.Fatalf("ceremony cookies = %#v", cookies)
	}
	assertInvalidRegistrationLeavesNoCredentials(t, app, session, cookies[0])
}

func assertInvalidRegistrationLeavesNoCredentials(t *testing.T, app testApplication, session testSession, cookie *http.Cookie) {
	t.Helper()
	for name, body := range map[string]string{
		"missing":     `{}`,
		"unknown":     `{"unexpected":true}`,
		"malformed":   `{"id":`,
		"oversized":   `{"credential":"` + strings.Repeat("x", 65<<10) + `"}`,
		"conflicting": `{"id":"a","ID":"b"}`,
	} {
		t.Run(name, func(t *testing.T) {
			finish := passkeyRequest(t, "/api/v1/passkeys/register/finish", body, &session)
			finish.AddCookie(cookie)
			finishResponse := httptest.NewRecorder()
			app.handler.ServeHTTP(finishResponse, finish)
			owner, ownerErr := app.auth.PasskeyOwner()
			if finishResponse.Code != http.StatusBadRequest || ownerErr != nil || len(owner.Credentials) != 0 {
				t.Fatalf("invalid finish = %d, passkeys = %d, owner error = %v", finishResponse.Code, len(owner.Credentials), ownerErr)
			}
		})
	}
}

func TestPasskeyLoginBeginCreatesDiscoverableChallenge(t *testing.T) {
	app := newTestApplication(t, Config{PublicURL: "https://dashboard.example:38400", SecureCookies: true, TrustedHosts: []string{"dashboard.example"}})
	request := passkeyRequest(t, "/api/v1/passkeys/login/begin", "", nil)
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"publicKey"`) {
		t.Fatalf("login begin = %d, %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("login ceremony cookies = %#v", cookies)
	}
}

func TestPasskeyLoginBeginDoesNotTreatConditionalOffersAsFailures(t *testing.T) {
	app := newTestApplication(t, Config{PublicURL: "https://dashboard.example:38400", SecureCookies: true, TrustedHosts: []string{"dashboard.example"}})
	for attempt := 1; attempt <= perSourceLogins+1; attempt++ {
		request := passkeyRequest(t, "/api/v1/passkeys/login/begin", "", nil)
		response := httptest.NewRecorder()
		app.handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("conditional offer %d status = %d, body = %s", attempt, response.Code, response.Body.String())
		}
	}
}

func passkeyRequest(t *testing.T, path, body string, session *testSession) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://dashboard.example:38400"+path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if session != nil {
		request.AddCookie(session.cookie)
		request.Header.Set("X-Kinosail-CSRF", session.csrf)
	}
	return request
}
