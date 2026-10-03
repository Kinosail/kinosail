package servertest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// AssertPasskeyCookieIsolation exercises signed enrollment and login through app handlers.
//
//nolint:cyclop,gocognit,funlen // The complete real-handler browser journey preserves its cookie and security assertions together.
func AssertPasskeyCookieIsolation(t *testing.T, handler http.Handler, origin, sessionName, ceremonyName, siblingCeremony string) {
	t.Helper()
	base, err := url.Parse(origin)
	if err != nil {
		t.Fatal(err)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	call := func(path, body, contentType string) *httptest.ResponseRecorder {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, origin+path, strings.NewReader(body))
		request.Header.Set("User-Agent", "Kinosail synthetic browser")
		request.Header.Set("Origin", origin)
		request.Header.Set("Sec-Fetch-Site", "same-origin")
		request.Header.Set("Content-Type", contentType)
		for _, cookie := range jar.Cookies(request.URL) {
			request.AddCookie(cookie)
			if cookie.Name == sessionName {
				request.Header.Set("X-Kinosail-CSRF", httpguard.CSRFToken(cookie.Value))
			}
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		jar.SetCookies(base, response.Result().Cookies())
		return response
	}
	setup := call("/setup", "name=Owner&password=owner-password", "application/x-www-form-urlencoded")
	if setup.Code != http.StatusSeeOther {
		t.Fatalf("setup status = %d", setup.Code)
	}
	begin := call("/api/v1/passkeys/register/begin", "", "application/json")
	challenge, user := passkeyOptions(t, begin, ceremonyName)
	device := NewWebAuthnDevice(t, "localhost", origin, "app-cookie-fixture-credential")
	registered := call("/api/v1/passkeys/register/finish", device.Registration(t, challenge), "application/json")
	if registered.Code != http.StatusNoContent {
		t.Fatalf("registration status = %d", registered.Code)
	}
	legacy := *setup.Result().Cookies()[0] //nolint:gosec // Preserve the real secure session response while changing only its legacy name.
	legacy.Name = httpguard.LegacySessionCookieName
	// Retain an earlier valid pre-upgrade cookie while creating a fresh app session.
	jar.SetCookies(base, []*http.Cookie{&legacy})
	begin = call("/api/v1/passkeys/login/begin", "", "application/json")
	challenge, _ = passkeyOptions(t, begin, ceremonyName)
	// Another app starts its own ceremony on the same hostname, on another port.
	jar.SetCookies(base, []*http.Cookie{{Name: siblingCeremony, Value: "sibling-ceremony-fixture", Path: "/api/v1/passkeys/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: 60}})
	signedIn := call("/api/v1/passkeys/login/finish", device.Assertion(t, challenge, user), "application/json")
	if signedIn.Code != http.StatusNoContent {
		t.Fatalf("signed passkey login after sibling ceremony = %d", signedIn.Code)
	}
	found := false
	for _, cookie := range signedIn.Result().Cookies() {
		if cookie.Name == sessionName && cookie.Secure && cookie.HttpOnly && cookie.SameSite == http.SameSiteStrictMode && cookie.MaxAge == 8*60*60 && !cookie.Expires.IsZero() {
			found = true
		}
	}
	if !found {
		t.Fatal("passkey login did not issue the persistent app-specific session")
	}

	if response := call("/logout", "", "application/x-www-form-urlencoded"); response.Code != http.StatusSeeOther {
		t.Fatalf("logout status = %d", response.Code)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, origin+"/account", nil)
	request.AddCookie(&legacy)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/login" {
		t.Fatal("logout resurrected the old legacy sign-in")
	}
}

func passkeyOptions(t *testing.T, response *httptest.ResponseRecorder, cookieName string) (string, string) {
	t.Helper()
	var options struct {
		PublicKey struct {
			Challenge string
			User      struct{ ID string }
		}
	}
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &options) != nil || options.PublicKey.Challenge == "" {
		t.Fatalf("passkey begin status = %d", response.Code)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != cookieName || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("passkey ceremony was not app-scoped and secure")
	}
	user, err := base64.RawURLEncoding.DecodeString(options.PublicKey.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	return options.PublicKey.Challenge, string(user)
}
