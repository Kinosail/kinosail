package jellyfincompat

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

type identityProfile struct {
	id, name string
	mfa      bool
}

func identityFixture(effects *[]string) Identity[identityProfile] {
	return Identity[identityProfile]{
		ServerID: "server", ServerName: func() string { return "Player" }, Configured: func() bool { return true },
		Secure:    func(request *http.Request) bool { return request.Header.Get("X-Secure") == "true" },
		Public:    func(*http.Request) bool { return false },
		Current:   func(*http.Request) identityProfile { return identityProfile{id: "viewer", name: "Sam"} },
		ProfileID: func(profile identityProfile) string { return profile.id },
		Project: func(profile identityProfile) User {
			return User{ID: profile.id, Name: profile.name, ServerID: "server"}
		},
		AllowLogin: func(_, username string) bool {
			*effects = append(*effects, "rate")
			return username != "limited"
		},
		Authenticate: func(username, password string) (identityProfile, bool) {
			*effects = append(*effects, "authenticate")
			return identityProfile{id: "viewer", name: username, mfa: username == "mfa"}, password == "secret"
		},
		RequiresMFA: func(profile identityProfile) bool { return profile.mfa },
		Compatibility: func(profile identityProfile) identityProfile {
			*effects = append(*effects, "compatibility")
			return profile
		},
		Audit:          func(*http.Request, identityProfile) { *effects = append(*effects, "audit") },
		LoginSucceeded: func(string) { *effects = append(*effects, "success") },
		CreateSession: func(id, _ string) (string, error) {
			*effects = append(*effects, "session")
			if id == "fail" {
				return "", errors.New("persist")
			}
			return "token", nil
		},
		SignOut: func(*http.Request) error {
			*effects = append(*effects, "signout")
			return nil
		},
	}
}

func TestIdentityOwnsServerUserAndLogoutHTTP(t *testing.T) { //nolint:cyclop // One protocol fixture checks all identity route fields and side effects.
	t.Parallel()
	effects := make([]string, 0)
	identity := identityFixture(&effects)
	for secure, address := range map[bool]string{false: "http://example.test", true: "https://example.test"} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/System/Info", nil)
		request.Host = "example.test"
		request.Header.Set("X-Secure", map[bool]string{true: "true"}[secure])
		response := httptest.NewRecorder()
		identity.SystemInfo(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"LocalAddress":"`+address+`"`) {
			t.Fatalf("system info = %d %q", response.Code, response.Body.String())
		}
	}
	for name, id := range map[string]string{"current": "", "matching": "viewer", "mismatch": "other", "control": "bad\n", "unicode control": "bad\u0085", "oversized": strings.Repeat("x", 257)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/Users/value", nil)
		request.SetPathValue("id", id)
		response := httptest.NewRecorder()
		identity.User(response, request)
		if rejected := name != "current" && name != "matching"; rejected != (response.Code == http.StatusNotFound) {
			t.Fatalf("%s user = %d %q", name, response.Code, response.Body.String())
		}
	}
	logout := httptest.NewRecorder()
	identity.Logout(logout, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/Sessions/Logout", nil))
	if logout.Code != http.StatusNoContent || effects[len(effects)-1] != "signout" {
		t.Fatalf("logout = %d effects=%v", logout.Code, effects)
	}
	identity.SignOut = func(*http.Request) error { return errors.New("persist") }
	failed := httptest.NewRecorder()
	identity.Logout(failed, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	if failed.Code != http.StatusInternalServerError || failed.Body.String() != "could not end session\n" {
		t.Fatalf("failed logout = %d %q", failed.Code, failed.Body.String())
	}
}

func TestIdentityPasswordLoginPreservesOrderingAndErrors(t *testing.T) { //nolint:cyclop // One table protects all ordered exits.
	t.Parallel()
	for _, test := range []struct {
		username, password string
		wantErr            error
		wantEffects        string
	}{
		{"limited", "secret", ErrCredentialRateLimit, "rate"},
		{"Sam", "wrong", ErrInvalidCredentials, "rate,authenticate"},
		{"mfa", "secret", ErrQuickConnectMFA, "rate,authenticate"},
		{"Sam", "secret", nil, "rate,authenticate,compatibility,audit,success,session"},
	} {
		effects := make([]string, 0)
		identity := identityFixture(&effects)
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/login", nil)
		result, err := identity.PasswordLogin(request, Credentials{Username: test.username, Pw: test.password})
		if !errors.Is(err, test.wantErr) || strings.Join(effects, ",") != test.wantEffects || err == nil && (result.Token != "token" || result.User.Name != test.username) {
			t.Fatalf("%s = %#v, %v effects=%v", test.username, result, err, effects)
		}
	}
	effects := make([]string, 0)
	identity := identityFixture(&effects)
	identity.ProfileID = func(identityProfile) string { return "fail" }
	if _, err := identity.PasswordLogin(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), Credentials{Username: "Sam", Pw: "secret"}); err == nil {
		t.Fatal("session failure was hidden")
	}
}

func TestQuickConnectAdaptersValidateBeforeMutation(t *testing.T) { //nolint:cyclop,funlen,gocognit // Negative inputs share one no-side-effect assertion.
	t.Parallel()
	profile := identityProfile{id: "viewer", name: "Sam"}
	approvedSecret := "qc_AAAAAAAAAAAAAAAAAAAAAAAAAA"
	missingSecret := "qc_BBBBBBBBBBBBBBBBBBBBBBBBBB"
	approvals := 0
	handler := QuickConnectApproval(
		func(*http.Request) bool { return false }, func(*http.Request) identityProfile { return profile },
		func(profile identityProfile) string { return profile.id }, func(*http.Request) bool { return true },
		func(identityProfile, string, bool) error { approvals++; return nil },
	)
	for _, target := range []string{
		"/QuickConnect/Authorize?userId=other&code=123456",
		"/QuickConnect/Authorize?userId=viewer&userId=viewer&code=123456",
		"/QuickConnect/Authorize?userId=viewer&code=abcdef",
		"/QuickConnect/Authorize?userId=viewer&code=" + strings.Repeat("1", 65),
	} {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, nil))
		want := http.StatusForbidden
		if strings.Contains(target, strings.Repeat("1", 65)) || strings.Contains(target, "abcdef") {
			want = http.StatusNotFound
		}
		if response.Code != want || approvals != 0 {
			t.Fatalf("invalid approval = %d effects=%d", response.Code, approvals)
		}
	}
	response := httptest.NewRecorder()
	handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Authorize?userId=viewer&code=123456", nil))
	if response.Code != http.StatusOK || approvals != 1 {
		t.Fatalf("approval = %d effects=%d", response.Code, approvals)
	}
	public := QuickConnectApproval(func(*http.Request) bool { return true }, func(*http.Request) identityProfile { return profile }, func(identityProfile) string { return "viewer" }, func(*http.Request) bool { return true }, func(identityProfile, string, bool) error { approvals++; return nil })
	publicResponse := httptest.NewRecorder()
	public(publicResponse, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/QuickConnect/Authorize?code=123456", nil))
	if publicResponse.Code != http.StatusNotFound || approvals != 1 {
		t.Fatalf("public approval = %d effects=%d", publicResponse.Code, approvals)
	}

	audits := 0
	consumes := 0
	authenticate := QuickConnectAuthentication(&httpguard.Limiter{}, func(secret string) (string, identityProfile, error) {
		consumes++
		if secret != approvedSecret {
			return "", identityProfile{}, errors.New("missing")
		}
		return "token", profile, nil
	}, func(*http.Request, identityProfile) { audits++ }, func(profile identityProfile) User { return User{ID: profile.id, Name: profile.name} })
	missing := httptest.NewRecorder()
	authenticate(missing, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"Secret":"`+missingSecret+`"}`)))
	for _, body := range []string{`{"Secret":""}`, `{"Secret":"wrong"}`, `{"Secret":"qc_lower"}`, `{"Secret":"` + strings.Repeat("x", 129) + `"}`} {
		invalid := httptest.NewRecorder()
		authenticate(invalid, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(body)))
		if invalid.Code != http.StatusNotFound || consumes != 1 {
			t.Fatalf("invalid secret = %d consumes=%d", invalid.Code, consumes)
		}
	}
	success := httptest.NewRecorder()
	authenticate(success, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{"Secret":"`+approvedSecret+`"}`)))
	if missing.Code != http.StatusNotFound || success.Code != http.StatusOK || audits != 1 || consumes != 2 {
		t.Fatalf("authentication = %d/%d audits=%d consumes=%d", missing.Code, success.Code, audits, consumes)
	}
	if _, valid := StrictQuery(map[string][]string{"Secret": {"one"}, "secret": {"two"}}, "secret", 8); valid {
		t.Fatal("conflicting query was accepted")
	}
}
