package identitycore

import (
	"errors"
	"net/http"
	"time"
)

const MaxSessionTokenBytes = 256

var (
	ErrSessionKind  = errors.New("session kind is invalid")
	ErrSessionToken = errors.New("session token is invalid")
)

// RequestSessions owns Player's session kinds and HTTP cookie lifecycle.
type RequestSessions struct {
	*Sessions
	token  func(*http.Request) string
	source func(*http.Request) string
}

// NewRequestSessions binds the core session store to Player's token precedence.
func NewRequestSessions(config SessionConfig, token func(*http.Request) string, sources ...func(*http.Request) string) *RequestSessions {
	source := func(request *http.Request) string { return SessionTokenSource(request, nil) }
	if len(sources) > 0 && sources[0] != nil {
		source = sources[0]
	}
	return &RequestSessions{Sessions: NewSessions(config), token: token, source: source}
}

// CreateCompatibility issues one local compatibility session.
func (sessions *RequestSessions) CreateCompatibility(profileID, name string) (string, error) {
	return sessions.core().Create(profileID, name, false, false, "compatibility")
}

// CreateLocal issues one local session with explicit browser and strength state.
func (sessions *RequestSessions) CreateLocal(profileID, name string, browser, strong bool) (string, error) {
	return sessions.core().Create(profileID, name, browser, strong, "")
}

// CreateStrong issues one strong local session.
func (sessions *RequestSessions) CreateStrong(profileID, name string, browser bool) (string, error) {
	return sessions.core().Create(profileID, name, browser, true, "")
}

// CreateStrongPublic issues one strong, short-lived public session.
func (sessions *RequestSessions) CreateStrongPublic(profileID, name string, browser bool) (string, error) {
	return sessions.core().Create(profileID, name, browser, true, "public")
}

// SignIn creates one browser session before it writes the secure cookie.
func (sessions *RequestSessions) SignIn(writer http.ResponseWriter, request *http.Request, profileID string) error {
	return sessions.signIn(writer, request, profileID, false, false)
}

// SignInStrong creates one strong browser session before it writes the secure cookie.
func (sessions *RequestSessions) SignInStrong(writer http.ResponseWriter, request *http.Request, profileID string) error {
	return sessions.signIn(writer, request, profileID, true, false)
}

// SignInStrongPublic creates one strong public session before it writes the secure cookie.
func (sessions *RequestSessions) SignInStrongPublic(writer http.ResponseWriter, request *http.Request, profileID string) error {
	return sessions.signIn(writer, request, profileID, true, true)
}

// SignInPublicGrant atomically checks a locally approved revision before issuing a cookie.
func (sessions *RequestSessions) SignInPublicGrant(writer http.ResponseWriter, request *http.Request, profileID string, revision uint64) error {
	if writer == nil || request == nil {
		return ErrInvalidConfig
	}
	if ManagementDeviceKey(request) != "" {
		return ErrSessionKind
	}
	token, err := sessions.core().CreatePublicGrant(profileID, request.UserAgent(), true, revision)
	if err != nil {
		return err
	}
	http.SetCookie(writer, sessions.issuedBrowserCookie(token, request))
	return nil
}

func (sessions *RequestSessions) signIn(writer http.ResponseWriter, request *http.Request, profileID string, strong, public bool) error {
	if writer == nil || request == nil {
		return ErrInvalidConfig
	}
	channel := ""
	if public {
		channel = "public"
	}
	token, err := sessions.CreateForRequest(request, profileID, request.UserAgent(), true, strong, channel)
	if err != nil {
		return err
	}
	http.SetCookie(writer, sessions.issuedBrowserCookie(token, request))
	return nil
}

// Use the committed expiry, including the public cap, rather than restarting the
// lifetime after persistence. A second boundary must not extend the cookie.
func (sessions *RequestSessions) issuedBrowserCookie(token string, request *http.Request) *http.Cookie {
	sessions.config.Mutex.RLock()
	expires := (*sessions.config.Values)[SessionKey(token)].ExpiresAt
	sessions.config.Mutex.RUnlock()
	cookie := SessionCookie(token, request) //nolint:gosec // SessionCookie always sets Secure, HttpOnly, and Strict SameSite.
	cookie.Expires = time.Unix(expires, 0)
	cookie.MaxAge = int(expires - sessions.config.Now().Unix())
	if cookie.MaxAge <= 0 {
		cookie.MaxAge = -1
	}
	return cookie
}

// MarkStrong records recent authentication for the request session.
func (sessions *RequestSessions) MarkStrong(request *http.Request) error {
	token, valid := sessions.requestToken(request)
	if !valid {
		return ErrCurrentSession
	}
	return sessions.core().MarkStrong(token)
}

// RecentlyAuthenticated reports recent strong authentication for the request session.
func (sessions *RequestSessions) RecentlyAuthenticated(request *http.Request, maximumAge time.Duration) bool {
	token, valid := sessions.requestToken(request)
	return valid && sessions.core().RecentlyAuthenticated(token, maximumAge)
}

// Public reports whether the request uses a public session.
func (sessions *RequestSessions) Public(request *http.Request) bool {
	token, valid := sessions.requestToken(request)
	return valid && sessions.core().Public(token)
}

// SignOut removes the request session before the caller clears its cookie.
func (sessions *RequestSessions) SignOut(request *http.Request) error {
	token, valid := sessions.requestToken(request)
	if !valid {
		return nil
	}
	return sessions.signOutBrowserTokens(request, token)
}

func (sessions *RequestSessions) requestToken(request *http.Request) (string, bool) {
	if sessions == nil || sessions.token == nil || request == nil {
		return "", false
	}
	token := sessions.token(request)
	if sessions.Sessions != nil && sessions.Sessions.valid() {
		sessions.config.Mutex.RLock()
		value, found := (*sessions.config.Values)[SessionKey(token)]
		sessions.config.Mutex.RUnlock()
		if found && !SessionMatchesRequest(value, request) {
			return "", false
		}
	}
	return token, ValidSessionToken(token)
}

func (sessions *RequestSessions) core() *Sessions {
	if sessions == nil {
		return nil
	}
	return sessions.Sessions
}

// ValidSessionToken accepts one bounded, unambiguous credential value.
func ValidSessionToken(token string) bool {
	if token == "" || len(token) > MaxSessionTokenBytes {
		return false
	}
	for _, value := range []byte(token) {
		if value <= ' ' || value > '~' {
			return false
		}
	}
	return true
}
