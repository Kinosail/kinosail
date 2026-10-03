package servertest

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

// AssertIssuedCookieLifetime binds real-clock cookie issuance to persisted expiry.
func AssertIssuedCookieLifetime(t *testing.T, cookie *http.Cookie, lifetime time.Duration, started, finished time.Time, persisted []byte) {
	t.Helper()
	assertIssuedCookieSecurity(t, cookie)
	var sessions map[string]identitycore.Session
	if json.Unmarshal(persisted, &sessions) != nil {
		t.Fatal("persisted browser sessions are malformed")
	}
	session, found := sessions[identitycore.SessionKey(cookie.Value)]
	if !found || !session.Browser {
		t.Fatal("issued browser cookie has no persisted browser session")
	}
	assertIssuedCookieClock(t, cookie, session, lifetime, started, finished)
}

func assertIssuedCookieClock(t *testing.T, cookie *http.Cookie, session identitycore.Session, lifetime time.Duration, started, finished time.Time) {
	t.Helper()
	seconds := int64(lifetime / time.Second)
	if session.CreatedAt < started.Unix() || session.CreatedAt > finished.Unix() || session.ExpiresAt != session.CreatedAt+seconds {
		t.Fatal("persisted browser session lifetime differs from the configured cap or request interval")
	}
	if cookie.Expires.Unix() != session.ExpiresAt {
		t.Fatal("issued browser cookie expiry differs from persisted expiry")
	}
	remaining := int64(cookie.MaxAge)
	if remaining <= 0 || remaining > seconds || remaining < session.ExpiresAt-finished.Unix() || remaining > session.ExpiresAt-started.Unix() {
		t.Fatalf("issued browser cookie remaining lifetime = %d; expiry = %d, request interval = %d..%d", remaining, session.ExpiresAt, started.Unix(), finished.Unix())
	}
}

func assertIssuedCookieSecurity(t *testing.T, cookie *http.Cookie) {
	t.Helper()
	if cookie == nil {
		t.Fatal("issued browser cookie is absent")
	}
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || cookie.Expires.IsZero() {
		t.Fatal("issued browser cookie lacks its host-bound security attributes")
	}
}
