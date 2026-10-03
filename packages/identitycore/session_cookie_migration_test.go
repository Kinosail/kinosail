package identitycore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

func scopedSessionRequest(t *testing.T) *http.Request {
	t.Helper()
	var request *http.Request
	httpguard.WithSessionCookieName(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { request = r }), "__Host-kinosail_player_session").ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://localhost:38127/account", nil))
	return request
}

//nolint:cyclop,gocognit // One lifecycle table checks each independent authorization and persistence boundary.
func TestLegacyBrowserCookieMigrationPreservesSessionLimits(t *testing.T) {
	t.Parallel()
	for _, public := range []bool{false, true} {
		fixture := newSessionFixture()
		sessions := requestSessionFixture(fixture)
		channel := ""
		if public {
			channel = "public"
		}
		token, err := sessions.CreateLocal("viewer", "Browser", true, true)
		if err != nil {
			t.Fatal(err)
		}
		state := fixture.values[SessionKey(token)]
		state.Channel = channel
		state.ExpiresAt = fixture.now.Add(2 * time.Hour).Unix()
		fixture.values[SessionKey(token)] = state
		fixture.now = fixture.now.Add(20 * time.Minute)
		request := scopedSessionRequest(t)
		request.AddCookie(SessionCookie(token))
		response := httptest.NewRecorder()
		sessions.MigrateBrowserCookie(response, request)
		cookies := response.Result().Cookies()
		if len(cookies) != 1 || cookies[0].Name != "__Host-kinosail_player_session" || cookies[0].Value != token || cookies[0].Expires.Unix() != state.ExpiresAt || cookies[0].MaxAge != 100*60 {
			t.Fatalf("migration cookie attributes = names %v, count %d", response.Header().Values("Set-Cookie") != nil, len(cookies))
		}
		if fixture.values[SessionKey(token)] != state || fixture.writes != 1 {
			t.Fatal("migration changed server session or authentication timestamps")
		}
		request = scopedSessionRequest(t)
		request.AddCookie(cookies[0])
		if SessionTokenSource(request, nil) != "kinosail-session-cookie" {
			t.Fatal("browser token source no longer protects playback URLs")
		}
	}
}

//nolint:cyclop,gocognit // One lifecycle table checks each independent authorization and persistence boundary.
func TestLegacyBrowserCookieMigrationRejectsUnselectedOrInvalidSessions(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"expired", "idle", "revoked", "disabled", "device", "bearer", "scoped", "empty scoped", "management mismatch", "public revision"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newSessionFixture()
			sessions := requestSessionFixture(fixture)
			token, err := sessions.CreateLocal("viewer", "Browser", true, true)
			if err != nil {
				t.Fatal(err)
			}
			request := scopedSessionRequest(t)
			request.AddCookie(SessionCookie(token))
			state := fixture.values[SessionKey(token)]
			switch kind {
			case "expired":
				state.ExpiresAt = fixture.now.Unix()
			case "idle":
				state.LastSeen = fixture.now.Add(-2 * time.Hour).Unix()
			case "disabled":
				fixture.profiles[0].Disabled = true
			case "device":
				state.Browser = false
			case "bearer":
				request.Header.Set("Authorization", "Bearer elsewhere")
			case "scoped":
				request.AddCookie(&http.Cookie{Name: "__Host-kinosail_player_session", Value: "invalid-scoped", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			case "empty scoped":
				request.AddCookie(&http.Cookie{Name: "__Host-kinosail_player_session", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			case "management mismatch":
				state.ManagementDevice = "unmatched-device"
			case "public revision":
				state.Channel = "public"
				state.ProfileRevision++
			}
			fixture.values[SessionKey(token)] = state
			if kind == "revoked" {
				delete(fixture.values, SessionKey(token))
			}
			response := httptest.NewRecorder()
			sessions.MigrateBrowserCookie(response, request)
			if len(response.Result().Cookies()) != 0 || fixture.writes != 1 {
				t.Fatal("invalid or unselected legacy session was migrated")
			}
		})
	}
}

//nolint:cyclop,gocognit // One lifecycle table checks each independent authorization and persistence boundary.
func TestCookieLogoutRetiresLegacyWithoutAffectingOtherAuthentication(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"cookie", "bearer", "management mismatch", "persistence failure"} {
		t.Run(kind, func(t *testing.T) {
			fixture := newSessionFixture()
			sessions := requestSessionFixture(fixture)
			token, err := sessions.CreateLocal("viewer", "Browser", true, true)
			if err != nil {
				t.Fatal(err)
			}
			legacy := fixture.values[SessionKey(token)]
			fixture.values[SessionKey("legacy")] = legacy
			request := scopedSessionRequest(t)
			request.AddCookie(&http.Cookie{Name: "__Host-kinosail_player_session", Value: token, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			request.AddCookie(SessionCookie("legacy"))
			request.AddCookie(&http.Cookie{Name: "__Host-kinosail_subtitles_session", Value: "sibling", Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
			if kind == "bearer" {
				request.Header.Set("Authorization", "Bearer "+token)
			}
			if kind == "management mismatch" {
				legacy.ManagementDevice = "unmatched"
				fixture.values[SessionKey("legacy")] = legacy
			}
			if kind == "persistence failure" {
				fixture.persist = errors.New("fixture write failed")
			}
			err = sessions.SignOut(request)
			_, currentExists := fixture.values[SessionKey(token)]
			_, legacyExists := fixture.values[SessionKey("legacy")]
			if kind == "persistence failure" {
				if err == nil || !currentExists || !legacyExists {
					t.Fatal("failed logout changed session state")
				}
			} else if err != nil || currentExists || legacyExists != (kind != "cookie") {
				t.Fatal("logout session retirement mismatch")
			}
		})
	}
}
