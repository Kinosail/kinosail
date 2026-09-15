package identitycore

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func requestSessionFixture(fixture *sessionFixture) *RequestSessions {
	core := fixture.sessions()
	return NewRequestSessions(core.config, func(request *http.Request) string { return SessionToken(request, nil) })
}

func TestRequestSessionsCreateEveryPlayerSessionKind(t *testing.T) {
	t.Parallel()
	type expectedState struct {
		browser bool
		strong  bool
		channel string
	}
	for name, test := range map[string]struct {
		issue func(*RequestSessions) (string, error)
		want  expectedState
	}{
		"device": {
			issue: func(sessions *RequestSessions) (string, error) {
				return sessions.CreateLocal("viewer", "Device", false, false)
			},
		},
		"compatibility": {
			issue: func(sessions *RequestSessions) (string, error) {
				return sessions.CreateCompatibility("viewer", "Device")
			},
			want: expectedState{channel: "compatibility"},
		},
		"browser": {
			issue: func(sessions *RequestSessions) (string, error) {
				return sessions.CreateLocal("viewer", "Device", true, false)
			},
			want: expectedState{browser: true},
		},
		"strong device": {
			issue: func(sessions *RequestSessions) (string, error) {
				return sessions.CreateStrong("viewer", "Device", false)
			},
			want: expectedState{strong: true},
		},
		"public device": {
			issue: func(sessions *RequestSessions) (string, error) {
				return sessions.CreateStrongPublic("viewer", "Device", false)
			},
			want: expectedState{strong: true, channel: "public"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newSessionFixture()
			token, err := test.issue(requestSessionFixture(fixture))
			if err != nil || token != "token" || fixture.writes != 1 {
				t.Fatalf("issue = %q, %v writes=%d", token, err, fixture.writes)
			}
			state := fixture.values[SessionKey(token)]
			got := expectedState{browser: state.Browser, strong: state.StrongAt > 0, channel: state.Channel}
			if got != test.want {
				t.Fatalf("state = %#v, want %#v", got, test.want)
			}
		})
	}
	var missing *RequestSessions
	if _, err := missing.CreateLocal("viewer", "Device", false, false); !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("nil request sessions = %v", err)
	}
}

func TestWithoutPublicSessionsReturnsAnIndependentCopy(t *testing.T) {
	t.Parallel()
	original := map[string]Session{
		"public": {Channel: "public"},
		"local":  {ProfileID: "viewer"},
	}
	filtered := WithoutPublicSessions(original)
	delete(filtered, "local")
	if len(original) != 2 || len(filtered) != 0 {
		t.Fatalf("sessions changed: original=%#v filtered=%#v", original, filtered)
	}
}

func TestRequestSessionsRejectInvalidKindsAndTokensBeforePersistence(t *testing.T) {
	t.Parallel()
	fixture := newSessionFixture()
	if _, err := fixture.sessions().Create("viewer", "Device", false, false, "unknown"); !errors.Is(err, ErrSessionKind) || fixture.writes != 0 {
		t.Fatalf("unknown kind = %v writes=%d", err, fixture.writes)
	}
	for name, token := range map[string]string{
		"missing": "", "oversized": strings.Repeat("x", MaxSessionTokenBytes+1), "space": "two tokens", "control": "token\x7f",
	} {
		t.Run(name, func(t *testing.T) {
			candidate := newSessionFixture()
			_, err := candidate.sessions(func(config *SessionConfig) { config.NewToken = func() string { return token } }).Create("viewer", "Device", false, false, "")
			if !errors.Is(err, ErrSessionToken) || candidate.writes != 0 || len(candidate.values) != 0 {
				t.Fatalf("invalid token = %v writes=%d values=%#v", err, candidate.writes, candidate.values)
			}
		})
	}
	if !ValidSessionToken("Token-123_abc") {
		t.Fatal("bounded visible token was rejected")
	}
}

func TestRequestSessionsIssueCookiesAfterPersistence(t *testing.T) { //nolint:cyclop,gocognit // Exact cookie and state checks preserve all sign-in modes.
	t.Parallel()
	for name, signIn := range map[string]func(*RequestSessions, http.ResponseWriter, *http.Request) error{
		"browser": func(sessions *RequestSessions, writer http.ResponseWriter, request *http.Request) error {
			return sessions.SignIn(writer, request, "viewer")
		},
		"strong": func(sessions *RequestSessions, writer http.ResponseWriter, request *http.Request) error {
			return sessions.SignInStrong(writer, request, "viewer")
		},
		"public": func(sessions *RequestSessions, writer http.ResponseWriter, request *http.Request) error {
			return sessions.SignInStrongPublic(writer, request, "viewer")
		},
	} {
		t.Run(name, func(t *testing.T) {
			fixture := newSessionFixture()
			response := httptest.NewRecorder()
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
			request.Header.Set("User-Agent", "Mozilla Chrome/1 Safari/1")
			if err := signIn(requestSessionFixture(fixture), response, request); err != nil {
				t.Fatal(err)
			}
			cookies := response.Result().Cookies()
			state := fixture.values[SessionKey("token")]
			if fixture.writes != 1 || len(cookies) != 1 || cookies[0].Name != "__Host-kinosail_session" || cookies[0].Value != "token" || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || !state.Browser {
				t.Fatalf("%s cookie=%#v state=%#v writes=%d", name, cookies, state, fixture.writes)
			}
			if public := name == "public"; (state.Channel == "public") != public || (cookies[0].MaxAge > 0) != public {
				t.Fatalf("%s public state=%#v cookie=%#v", name, state, cookies[0])
			}
			if strong := name != "browser"; (state.StrongAt > 0) != strong {
				t.Fatalf("%s strong state=%#v", name, state)
			}
		})
	}
	fixture := newSessionFixture()
	fixture.persist = errors.New("persist failed")
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
	if err := requestSessionFixture(fixture).SignIn(response, request, "viewer"); err == nil || response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("failed persistence wrote cookie: %q, %v", response.Header().Get("Set-Cookie"), err)
	}
	if err := requestSessionFixture(newSessionFixture()).SignIn(nil, request, "viewer"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil writer = %v", err)
	}
	if err := requestSessionFixture(newSessionFixture()).SignIn(response, nil, "viewer"); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("nil request = %v", err)
	}
}

func TestRequestSessionsBindAuthenticationToValidatedRequestTokens(t *testing.T) { //nolint:cyclop // One request lifecycle covers every accepted and rejected token source.
	t.Parallel()
	fixture := newSessionFixture()
	sessions := requestSessionFixture(fixture)
	token, err := sessions.CreateStrongPublic("viewer", "Device", false)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/account", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	if err := sessions.MarkStrong(request); err != nil || !sessions.RecentlyAuthenticated(request, time.Minute) || !sessions.Public(request) {
		t.Fatalf("request session was not authenticated: %v", err)
	}
	if err := sessions.SignOut(request); err != nil || len(fixture.values) != 0 {
		t.Fatalf("sign out = %#v, %v", fixture.values, err)
	}

	for name, invalid := range map[string]func(*RequestSessions) *http.Request{
		"nil request": func(*RequestSessions) *http.Request { return nil },
		"nil token adapter": func(sessions *RequestSessions) *http.Request {
			sessions.token = nil
			return request
		},
		"malformed": func(*RequestSessions) *http.Request {
			value := request.Clone(request.Context())
			value.Header.Set("Authorization", "Bearer two tokens")
			return value
		},
		"oversized": func(*RequestSessions) *http.Request {
			value := request.Clone(request.Context())
			value.Header.Set("Authorization", "Bearer "+strings.Repeat("x", MaxSessionTokenBytes+1))
			return value
		},
	} {
		t.Run(name, func(t *testing.T) {
			candidate := newSessionFixture()
			bound := requestSessionFixture(candidate)
			input := invalid(bound)
			before := candidate.writes
			if !errors.Is(bound.MarkStrong(input), ErrCurrentSession) || bound.RecentlyAuthenticated(input, time.Hour) || bound.Public(input) || bound.SignOut(input) != nil || candidate.writes != before {
				t.Fatalf("invalid request changed sessions: writes=%d", candidate.writes)
			}
		})
	}
	var missing *RequestSessions
	if !errors.Is(missing.MarkStrong(request), ErrCurrentSession) || missing.RecentlyAuthenticated(request, time.Hour) || missing.Public(request) || missing.SignOut(request) != nil {
		t.Fatal("nil request lifecycle did not fail closed")
	}
	invalidCore := &RequestSessions{token: func(*http.Request) string { return "token" }}
	if !errors.Is(invalidCore.MarkStrong(request), ErrCurrentSession) || invalidCore.RecentlyAuthenticated(request, time.Hour) || invalidCore.Public(request) || !errors.Is(invalidCore.SignOut(request), ErrCurrentSession) {
		t.Fatal("nil core did not fail closed")
	}
}
