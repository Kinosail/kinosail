package identitycore

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/apihttp"
	"github.com/go-webauthn/webauthn/webauthn"
)

type apiSessionFixture struct {
	profile                      Profile
	public, allowed, validFactor bool
	sessionErr                   error
	credentialSuccess            int
	normalSessions, strong       int
	audits                       int
}

type apiSessionSuccessCase struct {
	name, contentType                  string
	profile                            Profile
	code, token                        string
	strong                             int
	enrollmentRequired, passkeyPresent bool
}

func (fixture *apiSessionFixture) config() APISessionConfig {
	return APISessionConfig{
		Public: func(*http.Request) bool { return fixture.public }, ReadJSON: apihttp.ReadJSON,
		AllowCredential: func(string, string) bool { return fixture.allowed },
		Authenticate: func(name, password string) (Profile, bool) {
			return fixture.profile, name == fixture.profile.Name && password == "password"
		},
		VerifySecondFactor:  func(string, string) bool { return fixture.validFactor },
		CredentialSucceeded: func(string) { fixture.credentialSuccess++ },
		CreateSession: func(string, string) (string, error) {
			fixture.normalSessions++
			return "normal-token", fixture.sessionErr
		},
		CreateStrongSession: func(string, string, bool) (string, error) {
			fixture.strong++
			return "strong-token", fixture.sessionErr
		},
		MFARequired: func(profile Profile) bool { return profile.Owner },
		SetAudit:    func(*http.Request, Profile) { fixture.audits++ },
		Error:       apihttp.Error, JSON: apihttp.WriteJSON,
	}
}

func apiSessionRequest(t *testing.T, contentType string, values url.Values) *http.Request {
	t.Helper()
	body := values.Encode()
	if strings.HasPrefix(contentType, "application/json") {
		body = `{"name":"` + values.Get("name") + `","password":"` + values.Get("password") + `","device":"` + values.Get("device") + `","code":"` + values.Get("code") + `"}`
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/session", strings.NewReader(body))
	request.Header.Set("Content-Type", contentType)
	request.RemoteAddr = "192.0.2.1:1234"
	return request
}

func TestCreateAPISessionAcceptsFormJSONAndMFA(t *testing.T) {
	for _, test := range []apiSessionSuccessCase{
		{"form", "application/x-www-form-urlencoded", Profile{ID: "viewer", Name: "Viewer"}, "", "strong-token", 1, false, false},
		{"JSON MFA", "application/json; charset=utf-8", Profile{ID: "owner", Name: "Owner", Owner: true, TOTPSecret: "secret"}, "123456", "strong-token", 1, false, false},
		{"owner enrollment", "application/x-www-form-urlencoded", Profile{ID: "owner", Name: "Owner", Owner: true}, "", "strong-token", 1, true, false},
		{"passkey", "application/x-www-form-urlencoded", Profile{ID: "viewer", Name: "Viewer", Passkeys: []webauthn.Credential{{}}}, "", "strong-token", 1, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &apiSessionFixture{profile: test.profile, allowed: true, validFactor: true}
			values := url.Values{"name": {test.profile.Name}, "password": {"password"}, "device": {"TV"}, "code": {test.code}}
			response := httptest.NewRecorder()
			CreateAPISession(response, apiSessionRequest(t, test.contentType, values), fixture.config())
			assertAPISessionSuccess(t, response, fixture, test)
		})
	}
}

func TestCreateAPISessionRejectsBeforeSessionEffects(t *testing.T) {
	base := url.Values{"name": {"Viewer"}, "password": {"password"}, "device": {"TV"}}
	for _, test := range []struct {
		name       string
		configure  func(*apiSessionFixture)
		values     url.Values
		content    string
		wantStatus int
	}{
		{"public", func(f *apiSessionFixture) { f.public = true }, base, "application/x-www-form-urlencoded", http.StatusForbidden},
		{"limited", func(f *apiSessionFixture) { f.allowed = false }, base, "application/x-www-form-urlencoded", http.StatusTooManyRequests},
		{"credentials", func(f *apiSessionFixture) {}, url.Values{"name": {"Viewer"}, "password": {"wrong"}}, "application/x-www-form-urlencoded", http.StatusUnauthorized},
		{"missing MFA", func(f *apiSessionFixture) { f.profile.TOTPSecret = "secret" }, base, "application/x-www-form-urlencoded", http.StatusUnauthorized},
		{"invalid MFA", func(f *apiSessionFixture) { f.profile.TOTPSecret = "secret" }, url.Values{"name": {"Viewer"}, "password": {"password"}, "code": {"bad"}}, "application/x-www-form-urlencoded", http.StatusUnauthorized},
		{"invalid JSON", func(f *apiSessionFixture) {}, base, "application/json", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &apiSessionFixture{profile: Profile{ID: "viewer", Name: "Viewer"}, allowed: true}
			test.configure(fixture)
			request := rejectedAPISessionRequest(t, test.name, test.content, test.values)
			response := httptest.NewRecorder()
			CreateAPISession(response, request, fixture.config())
			assertAPISessionRejected(t, response, fixture, test.wantStatus)
			if test.name == "limited" && response.Header().Get("Retry-After") != "60" {
				t.Fatal("rate-limit response omitted Retry-After")
			}
			if test.name == "missing MFA" && !strings.Contains(response.Body.String(), `"mfaRequired":true`) {
				t.Fatalf("MFA challenge = %s", response.Body.String())
			}
		})
	}
}

func assertAPISessionSuccess(t *testing.T, response *httptest.ResponseRecorder, fixture *apiSessionFixture, test apiSessionSuccessCase) {
	t.Helper()
	var result struct {
		Token                 string `json:"token"`
		ExpiresIn             int    `json:"expiresIn"`
		MFAEnrollmentRequired bool   `json:"mfaEnrollmentRequired"`
		Passkey               struct {
			Configured, UsedForSignIn bool
		} `json:"passkey"`
	}
	if response.Code != http.StatusCreated || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Token != test.token || result.ExpiresIn != 2592000 || result.MFAEnrollmentRequired != test.enrollmentRequired || result.Passkey.Configured != test.passkeyPresent || result.Passkey.UsedForSignIn {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	assertAPISessionEffects(t, fixture, test.strong)
}

func assertAPISessionEffects(t *testing.T, fixture *apiSessionFixture, strong int) {
	t.Helper()
	if fixture.credentialSuccess != 1 || fixture.audits != 1 || fixture.strong != strong || fixture.normalSessions != 1-strong {
		t.Fatalf("effects = %#v", fixture)
	}
}

func rejectedAPISessionRequest(t *testing.T, name, content string, values url.Values) *http.Request {
	t.Helper()
	request := apiSessionRequest(t, content, values)
	if name == "invalid JSON" {
		request.Body = io.NopCloser(strings.NewReader(`{"unknown":true}`))
	}
	return request
}

func assertAPISessionRejected(t *testing.T, response *httptest.ResponseRecorder, fixture *apiSessionFixture, wantStatus int) {
	t.Helper()
	if response.Code != wantStatus || fixture.credentialSuccess != 0 || fixture.normalSessions != 0 || fixture.strong != 0 || fixture.audits != 0 {
		t.Fatalf("response/effects = %d %s %#v", response.Code, response.Body.String(), fixture)
	}
}

func TestCreateAPISessionReportsPersistenceFailure(t *testing.T) {
	for _, profile := range []Profile{{ID: "viewer", Name: "Viewer"}, {ID: "viewer", Name: "Viewer", TOTPSecret: "secret"}} {
		fixture := &apiSessionFixture{profile: profile, allowed: true, validFactor: true, sessionErr: errors.New("persist failed")}
		response := httptest.NewRecorder()
		values := url.Values{"name": {"Viewer"}, "password": {"password"}, "code": {"123456"}}
		CreateAPISession(response, apiSessionRequest(t, "application/x-www-form-urlencoded", values), fixture.config())
		wantStrong := 0
		if profile.TOTPSecret != "" {
			wantStrong = 1
		}
		if response.Code != http.StatusInternalServerError || fixture.credentialSuccess != 1 || fixture.normalSessions != 1-wantStrong || fixture.strong != wantStrong || fixture.audits != 0 {
			t.Fatalf("response/effects = %d %s %#v", response.Code, response.Body.String(), fixture)
		}
	}
}

func TestCreateAPISessionRejectsIncompleteConfig(t *testing.T) {
	for _, test := range []struct {
		name  string
		clear func(*APISessionConfig)
	}{
		{"public", func(config *APISessionConfig) { config.Public = nil }},
		{"JSON reader", func(config *APISessionConfig) { config.ReadJSON = nil }},
		{"credential limiter", func(config *APISessionConfig) { config.AllowCredential = nil }},
		{"authentication", func(config *APISessionConfig) { config.Authenticate = nil }},
		{"second factor", func(config *APISessionConfig) { config.VerifySecondFactor = nil }},
		{"credential success", func(config *APISessionConfig) { config.CredentialSucceeded = nil }},
		{"session", func(config *APISessionConfig) { config.CreateSession = nil }},
		{"strong session", func(config *APISessionConfig) { config.CreateStrongSession = nil }},
		{"MFA policy", func(config *APISessionConfig) { config.MFARequired = nil }},
		{"audit", func(config *APISessionConfig) { config.SetAudit = nil }},
		{"error writer", func(config *APISessionConfig) { config.Error = nil }},
		{"JSON writer", func(config *APISessionConfig) { config.JSON = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := &apiSessionFixture{profile: Profile{ID: "viewer", Name: "Viewer"}, allowed: true}
			config := fixture.config()
			test.clear(&config)
			response := httptest.NewRecorder()
			CreateAPISession(response, apiSessionRequest(t, "application/x-www-form-urlencoded", url.Values{"name": {"Viewer"}, "password": {"password"}}), config)
			if response.Code != http.StatusInternalServerError || fixture.credentialSuccess != 0 || fixture.normalSessions != 0 || fixture.strong != 0 || fixture.audits != 0 {
				t.Fatalf("response/effects = %d %s %#v", response.Code, response.Body.String(), fixture)
			}
		})
	}
}
