package quickconnect

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/identitycore"
)

func browserCall(t *testing.T, handler http.HandlerFunc, cookie *http.Cookie, change func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://family.duckdns.org/auth/quick-connect", nil)
	request.Header.Set("Origin", "https://family.duckdns.org")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if change != nil {
		change(request)
	}
	response := httptest.NewRecorder()
	identitycore.Remote(handler).ServeHTTP(response, request)
	return response
}

func TestBrowserQuickConnectIssuesOnlyAnApprovedPublicCookie(t *testing.T) {
	t.Parallel()
	f := newApplicationFixture(t)
	started := browserCall(t, f.application.StartBrowser, nil, nil)
	var state struct {
		Code      string
		ExpiresIn int
	}
	if started.Code != http.StatusCreated || json.Unmarshal(started.Body.Bytes(), &state) != nil || len(state.Code) != 6 || state.ExpiresIn != 60 {
		t.Fatalf("start = %d %s", started.Code, started.Body.String())
	}
	cookies := started.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("start cookies = %v", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != browserCookieName || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" || cookie.MaxAge != 60 || strings.Contains(started.Body.String(), cookie.Value) {
		t.Fatalf("unsafe cookie or response: %#v", cookie)
	}
	if response := browserCall(t, f.application.PollBrowser, cookie, nil); response.Code != http.StatusAccepted || f.writes != 0 || len(response.Result().Cookies()) != 0 {
		t.Fatalf("pending = %d, writes=%d", response.Code, f.writes)
	}
	if err := f.application.Approve(f.current, state.Code, false); !errors.Is(err, ErrRemoteViewer) {
		t.Fatalf("weak approval = %v", err)
	}
	if err := f.application.Approve(identitycore.Profile{ID: "owner", Owner: true}, state.Code, true); !errors.Is(err, ErrRemoteViewer) {
		t.Fatalf("Owner approval = %v", err)
	}
	if err := f.application.Approve(f.current, state.Code, true); err != nil {
		t.Fatal(err)
	}
	connected := browserCall(t, f.application.PollBrowser, cookie, nil)
	if connected.Code != http.StatusNoContent || connected.Body.Len() != 0 || f.writes != 1 {
		t.Fatalf("connect = %d %s writes=%d", connected.Code, connected.Body.String(), f.writes)
	}
	var sessionCookie *http.Cookie
	for _, c := range connected.Result().Cookies() {
		if c.Name == "__Host-kinosail_session" {
			sessionCookie = c
		}
	}
	if sessionCookie == nil || !sessionCookie.HttpOnly || !sessionCookie.Secure || sessionCookie.MaxAge != 28800 || sessionCookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie = %#v", sessionCookie)
	}
	session := f.sessions[identitycore.SessionKey(sessionCookie.Value)]
	if session.Channel != "public" || !session.Browser || session.StrongAt == 0 || session.ExpiresAt-session.CreatedAt != 28800 {
		t.Fatalf("session = %#v", session)
	}
	if replay := browserCall(t, f.application.PollBrowser, cookie, nil); replay.Code != http.StatusNotFound || f.writes != 1 {
		t.Fatalf("replay = %d writes=%d", replay.Code, f.writes)
	}
}

func TestBrowserQuickConnectRejectsBadRequestsBeforeSideEffects(t *testing.T) {
	t.Parallel()
	cases := map[string]func(*http.Request){
		"missing origin":       func(r *http.Request) { r.Header.Del("Origin") },
		"null origin":          func(r *http.Request) { r.Header.Set("Origin", "null") },
		"foreign origin":       func(r *http.Request) { r.Header.Set("Origin", "https://attacker.example") },
		"insecure origin":      func(r *http.Request) { r.Header.Set("Origin", "http://family.duckdns.org") },
		"duplicate origin":     func(r *http.Request) { r.Header.Add("Origin", "https://family.duckdns.org") },
		"oversized origin":     func(r *http.Request) { r.Header.Set("Origin", strings.Repeat("a", 2048)) },
		"cross-site":           func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") },
		"unknown fetch site":   func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "unknown") },
		"query secret":         func(r *http.Request) { r.URL.RawQuery = "secret=chosen" },
		"empty query":          func(r *http.Request) { r.URL.ForceQuery = true },
		"wrong method":         func(r *http.Request) { r.Method = http.MethodGet },
		"unknown content type": func(r *http.Request) { r.Header.Set("Content-Type", "application/json") },
		"body":                 func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(`{"secret":"chosen"}`)) },
		"oversized body":       func(r *http.Request) { r.Body = io.NopCloser(strings.NewReader(strings.Repeat("x", 2048))) },
		"duplicate cookie":     func(r *http.Request) { r.AddCookie(&http.Cookie{Name: browserCookieName, Value: "other"}) },
		"oversized cookie":     func(r *http.Request) { r.Header.Set("Cookie", browserCookieName+"="+strings.Repeat("x", 129)) },
		"empty cookie":         func(r *http.Request) { r.Header.Set("Cookie", browserCookieName+"=") },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			for _, operation := range []string{"start", "poll", "cancel"} {
				f := newApplicationFixture(t)
				secret := f.approved(t, Request{Remote: true}, f.current)
				cookie := browserCookie(secret, 60)
				handler := map[string]http.HandlerFunc{"start": f.application.StartBrowser, "poll": f.application.PollBrowser, "cancel": f.application.CancelBrowser}[operation]
				response := browserCall(t, handler, cookie, change)
				if response.Code < 400 || response.Header().Get("Set-Cookie") != "" || f.writes != 0 || len(f.broker.pending) != 1 {
					t.Fatalf("%s accepted invalid request: %d, writes=%d pending=%d", operation, response.Code, f.writes, len(f.broker.pending))
				}
				if _, found := f.broker.Status(secret); !found {
					t.Fatalf("%s consumed or canceled the request", operation)
				}
			}
		})
	}
}

func TestBrowserQuickConnectRechecksEligibilityAndFailsClosed(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"local grant", "missing", "disabled", "deleted", "owner", "remote off", "unsecured", "revision", "expired", "revoked", "persistence"} {
		t.Run(failure, func(t *testing.T) {
			f := newApplicationFixture(t)
			secret := f.approved(t, Request{Remote: failure != "local grant"}, f.current)
			profile := f.current
			switch failure {
			case "missing":
				delete(f.profiles, "viewer")
			case "disabled":
				profile.Disabled = true
			case "deleted":
				profile.SCIMDeleted = true
			case "owner":
				profile.Owner = true
			case "remote off":
				profile.Remote = false
			case "unsecured":
				profile.TOTPSecret = ""
			case "revision":
				profile.Revision++
			case "expired":
				f.broker.now = func() time.Time { return time.Now().Add(time.Hour) }
			case "revoked":
				f.application.RevokeRemote()
			case "persistence":
				f.persistErr = errors.New("private disk failure")
			}
			if failure != "missing" {
				f.profiles["viewer"] = profile
			}
			response := browserCall(t, f.application.PollBrowser, browserCookie(secret, 60), nil)
			if response.Code < 400 || response.Header().Get("Set-Cookie") != "" || len(f.sessions) != 0 || strings.Contains(response.Body.String(), "private disk") {
				t.Fatalf("%s = %d %s sessions=%v", failure, response.Code, response.Body.String(), f.sessions)
			}
		})
	}
}

func TestBrowserQuickConnectCancellationReplacementAndLocalDenial(t *testing.T) {
	t.Parallel()
	f := newApplicationFixture(t)
	old := f.approved(t, Request{Remote: true}, f.current)
	started := browserCall(t, f.application.StartBrowser, browserCookie(old, 60), nil)
	if started.Code != http.StatusCreated {
		t.Fatalf("replacement = %d", started.Code)
	}
	if _, found := f.broker.Status(old); found {
		t.Fatal("replaced request still active")
	}
	cookie := started.Result().Cookies()[0]
	for range 2 {
		if response := browserCall(t, f.application.CancelBrowser, cookie, nil); response.Code != http.StatusNoContent {
			t.Fatalf("cancel = %d", response.Code)
		}
	}
	if len(f.broker.pending) != 0 || f.writes != 0 {
		t.Fatal("cancel created a session or left a request")
	}
	for _, handler := range []http.HandlerFunc{f.application.StartBrowser, f.application.PollBrowser, f.application.CancelBrowser} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "https://family.duckdns.org/auth/quick-connect", nil)
		request.Header.Set("Origin", "https://family.duckdns.org")
		response := httptest.NewRecorder()
		handler(response, request)
		if response.Code != http.StatusNotFound || len(f.broker.pending) != 0 || f.writes != 0 {
			t.Fatalf("local browser endpoint = %d", response.Code)
		}
	}
}

func TestQuickConnectRevisionChangeDuringIssuanceCreatesNoSession(t *testing.T) {
	t.Parallel()
	for _, browser := range []bool{false, true} {
		t.Run(map[bool]string{false: "native", true: "browser"}[browser], func(t *testing.T) {
			f := newApplicationFixture(t)
			secret := f.approved(t, Request{Remote: true}, f.current)
			// Simulate revocation after the approval lookup but before session issuance.
			f.application.adapter.Profiles.Find = func(id string) (identitycore.Profile, bool) {
				before := f.profiles[id]
				after := before
				after.Revision++
				f.profiles[id] = after
				return before, true
			}
			if browser {
				response := browserCall(t, f.application.PollBrowser, browserCookie(secret, 60), nil)
				if response.Code != http.StatusConflict || response.Header().Get("Set-Cookie") != "" {
					t.Fatalf("revoked browser issuance = %d", response.Code)
				}
			} else if token, _, err := f.application.Consume(secret); err == nil || token != "" {
				t.Fatalf("revoked native issuance = %q %v", token, err)
			}
			if f.writes != 0 || f.tokens != 0 || len(f.sessions) != 0 {
				t.Fatalf("revoked grant caused a side effect: writes=%d tokens=%d", f.writes, f.tokens)
			}
		})
	}
}

func TestBrowserQuickConnectRateLimitsCreationAndPolling(t *testing.T) {
	t.Parallel()
	f := newApplicationFixture(t)
	for range 20 {
		if response := browserCall(t, f.application.StartBrowser, nil, nil); response.Code != http.StatusCreated {
			t.Fatalf("start = %d", response.Code)
		}
	}
	response := browserCall(t, f.application.StartBrowser, nil, nil)
	if response.Code != http.StatusTooManyRequests || len(f.broker.pending) != 20 || response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("start rate limit = %d", response.Code)
	}
	secret, _, err := f.broker.Create(Request{Remote: true})
	if err != nil {
		t.Fatal(err)
	}
	for range 120 {
		if response = browserCall(t, f.application.PollBrowser, browserCookie(secret, 60), nil); response.Code != http.StatusAccepted {
			t.Fatalf("poll = %d", response.Code)
		}
	}
	response = browserCall(t, f.application.PollBrowser, browserCookie(secret, 60), nil)
	if response.Code != http.StatusTooManyRequests || f.writes != 0 || response.Header().Get("Set-Cookie") != "" {
		t.Fatalf("poll rate limit = %d", response.Code)
	}
}
