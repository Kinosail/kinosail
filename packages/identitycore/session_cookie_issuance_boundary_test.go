package identitycore

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The HTTP issuance seam uses a clock that crosses a second while persistence completes.
func TestCookieIssuancePreservesPersistedExpiryAcrossSecondBoundary(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"local", "public", "public-grant"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newSessionFixture()
			core := fixture.sessions()
			persist := core.config.Persist
			core.config.Persist = func(path string, value any) error {
				err := persist(path, value)
				fixture.now = fixture.now.Add(time.Second)
				return err
			}
			sessions := NewRequestSessions(core.config, func(r *http.Request) string { return SessionToken(r, nil) })
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
			var err error
			switch kind {
			case "local":
				err = sessions.SignInStrong(response, request, "viewer")
			case "public":
				err = sessions.SignInStrongPublic(response, request, "viewer")
			case "public-grant":
				err = sessions.SignInPublicGrant(response, request, "viewer", 7)
			}
			if err != nil {
				t.Fatal(err)
			}
			cookies := response.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("cookie count = %d", len(cookies))
			}
			state := fixture.values[SessionKey("token")]
			if cookies[0].Expires.Unix() != state.ExpiresAt || cookies[0].MaxAge != int(state.ExpiresAt-fixture.now.Unix()) {
				t.Fatal("issued cookie expiry differs from persisted session expiry")
			}
		})
	}
}
