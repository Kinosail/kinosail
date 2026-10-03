package identitycore

import (
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// MigrateBrowserCookie preserves a validated legacy session at the app's cookie name.
// It does not renew authentication, change server state, or delete a sibling's cookie.
func (sessions *RequestSessions) MigrateBrowserCookie(writer http.ResponseWriter, request *http.Request) {
	if writer == nil || sessions == nil || !sessions.valid() {
		return
	}
	cookie := sessions.selectedLegacyCookie(request)
	if cookie == nil {
		return
	}
	now := sessions.config.Now()
	session, valid := sessions.migrationSession(request, cookie.Value, now.Unix())
	if !valid {
		return
	}
	_, absolute := sessions.TimeoutsFor(session)
	expires := min(session.ExpiresAt, session.CreatedAt+int64(absolute/time.Second))
	migrated := SessionCookie(cookie.Value, request) //nolint:gosec // SessionCookie always sets Secure, HttpOnly, and Strict SameSite.
	migrated.Expires, migrated.MaxAge = time.Unix(expires, 0), int(expires-now.Unix())
	http.SetCookie(writer, migrated)
}

func (sessions *RequestSessions) selectedLegacyCookie(request *http.Request) *http.Cookie {
	if request == nil || (request.Method != http.MethodGet && request.Method != http.MethodHead) || sessions.source(request) != "kinosail-session-cookie" {
		return nil
	}
	cookie := httpguard.BrowserSessionCookie(request)
	if cookie == nil || cookie.Name != httpguard.LegacySessionCookieName || httpguard.SessionCookieName(request) == cookie.Name {
		return nil
	}
	return cookie
}

func (sessions *RequestSessions) migrationSession(request *http.Request, token string, now int64) (Session, bool) {
	sessions.config.Mutex.RLock()
	defer sessions.config.Mutex.RUnlock()
	session, found := (*sessions.config.Values)[SessionKey(token)]
	inactive, absolute := sessions.TimeoutsFor(session)
	if !found || !session.Browser || !SessionMatchesRequest(session, request) || SessionExpired(session, now, inactive, absolute) {
		return session, false
	}
	profile, profileFound := findSessionProfile(sessions.config.Profiles(), session.ProfileID)
	if !profileFound || profile.Disabled || profile.Deleted {
		return session, false
	}
	revisionBound := session.Channel == "public" || session.ManagementDevice != ""
	return session, !revisionBound || session.ProfileRevision == profile.Revision
}

func (sessions *RequestSessions) signOutBrowserTokens(request *http.Request, token string) error {
	legacy := ""
	if sessions.source != nil && sessions.source(request) == "kinosail-session-cookie" && httpguard.SessionCookieName(request) != httpguard.LegacySessionCookieName {
		if cookie, _ := request.Cookie(httpguard.LegacySessionCookieName); cookie != nil {
			legacy = cookie.Value
		}
	}
	return sessions.core().change(func(values map[string]Session) error {
		delete(values, SessionKey(token))
		if current, found := values[SessionKey(legacy)]; legacy != "" && found && SessionMatchesRequest(current, request) {
			delete(values, SessionKey(legacy))
		}
		return nil
	})
}
