package passkeys

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	engine, err := New(Config{DefaultURL: "http://localhost:38127", DisplayName: "Kinosail", CookieName: "kinosail_passkey"})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func TestEngineConfigurationAndProtocolOptions(t *testing.T) { //nolint:cyclop // Configuration and protocol guarantees are verified as one contract.
	for _, config := range []Config{
		{DefaultURL: "http://localhost", CookieName: "cookie"},
		{DefaultURL: "http://localhost", DisplayName: strings.Repeat("x", 101), CookieName: "cookie"},
		{DefaultURL: "http://localhost", DisplayName: "Kinosail"},
		{DefaultURL: "http://localhost", DisplayName: "Kinosail", CookieName: "bad cookie"},
	} {
		if _, err := New(config); !errors.Is(err, ErrInvalidConfig) {
			t.Fatalf("invalid config = %v", err)
		}
	}
	if _, err := New(Config{URL: "not-an-origin", DefaultURL: "http://localhost", DisplayName: "Kinosail", CookieName: "cookie"}); !errors.Is(err, ErrInvalidOrigin) {
		t.Fatalf("invalid URL = %v", err)
	}
	creationFailure := errors.New("creation failed")
	if _, err := New(Config{DefaultURL: "http://localhost", DisplayName: "Kinosail", CookieName: "cookie", newWebAuthn: func(*webauthn.Config) (*webauthn.WebAuthn, error) {
		return nil, creationFailure
	}}); !errors.Is(err, creationFailure) {
		t.Fatalf("WebAuthn creation = %v", err)
	}
	engine := testEngine(t)
	if engine.Origin().String() != "http://localhost:38127" || (*Engine)(nil).Origin().String() != "" {
		t.Fatal("engine origin is incorrect")
	}
	user := NewUser(struct{}{}, "owner", "Owner", nil)
	creation, cookie, err := engine.BeginRegistration(user, Ceremony{Identity: "owner", CookiePath: "/api/v1/passkeys/"})
	if err != nil || creation.Response.RelyingParty.ID != "localhost" || creation.Response.RelyingParty.Name != "Kinosail" || creation.Response.AuthenticatorSelection.UserVerification != protocol.VerificationRequired || cookie.Name != "kinosail_passkey" || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.MaxAge != 300 {
		t.Fatalf("registration = %#v, %#v, %v", creation, cookie, err)
	}
	assertion, loginCookie, err := engine.BeginLogin(Ceremony{CookiePath: "/auth/passkeys/", Secure: true})
	if err != nil || assertion.Response.RelyingPartyID != "localhost" || assertion.Response.UserVerification != protocol.VerificationRequired || !loginCookie.Secure || loginCookie.Path != "/auth/passkeys/" {
		t.Fatalf("login = %#v, %#v, %v", assertion, loginCookie, err)
	}
}

func TestEngineRejectsInvalidCeremoniesBeforeStateChanges(t *testing.T) {
	engine := testEngine(t)
	user := NewUser(struct{}{}, "owner", "Owner", nil)
	beginFailure := errors.New("begin failed")
	engine.beginRegistration = func(webauthn.User, ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error) {
		return nil, nil, beginFailure
	}
	if _, _, err := engine.BeginRegistration(user, Ceremony{Identity: "owner", CookiePath: "/"}); !errors.Is(err, beginFailure) {
		t.Fatalf("registration begin = %v", err)
	}
	engine.beginLogin = func(...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error) {
		return nil, nil, beginFailure
	}
	if _, _, err := engine.BeginLogin(Ceremony{CookiePath: "/"}); !errors.Is(err, beginFailure) {
		t.Fatalf("login begin = %v", err)
	}
	engine = testEngine(t)
	for index, ceremony := range []Ceremony{
		{},
		{Identity: "owner", CookiePath: "relative"},
		{Identity: "owner", CookiePath: "//outside"},
		{Identity: "owner", CookiePath: "/bad;path"},
		{Identity: strings.Repeat("i", maxUserHandle+1), CookiePath: "/"},
		{Identity: "owner", CookiePath: "/" + strings.Repeat("p", maxCookiePath)},
	} {
		if _, _, err := engine.BeginRegistration(user, ceremony); !errors.Is(err, ErrInvalidCeremony) {
			t.Fatalf("invalid registration %d = %v", index, err)
		}
	}
	if _, _, err := engine.BeginRegistration(nil, Ceremony{Identity: "owner", CookiePath: "/"}); !errors.Is(err, ErrInvalidCeremony) {
		t.Fatalf("nil user = %v", err)
	}
	if _, _, err := (*Engine)(nil).BeginRegistration(user, Ceremony{Identity: "owner", CookiePath: "/"}); !errors.Is(err, ErrInvalidCeremony) {
		t.Fatalf("nil engine = %v", err)
	}
	if _, _, err := engine.BeginLogin(Ceremony{Identity: "owner", CookiePath: "/"}); !errors.Is(err, ErrInvalidCeremony) {
		t.Fatalf("login identity = %v", err)
	}
	if _, _, err := (*Engine)(nil).BeginLogin(Ceremony{CookiePath: "/"}); !errors.Is(err, ErrInvalidCeremony) {
		t.Fatalf("nil login engine = %v", err)
	}
	if len(engine.ceremonies) != 0 {
		t.Fatalf("invalid ceremonies changed state: %d", len(engine.ceremonies))
	}
}

func TestCeremoniesAreBoundedOneUseAndPubliclyRevocable(t *testing.T) {
	engine := testEngine(t)
	now := time.Unix(1000, 0)
	engine.now = func() time.Time { return now }
	engine.ceremonies["expired"] = ceremonyState{session: webauthn.SessionData{Expires: now.Add(-time.Second)}}
	engine.ceremonies["public"] = ceremonyState{session: webauthn.SessionData{Expires: now.Add(time.Minute)}, public: true}
	engine.ceremonies["local"] = ceremonyState{session: webauthn.SessionData{Expires: now.Add(time.Minute)}}
	cookie, err := engine.start(Ceremony{CookiePath: "/"}, &webauthn.SessionData{Expires: now.Add(time.Minute)})
	if err != nil || len(engine.ceremonies) != 3 {
		t.Fatalf("start = %#v, %v, count %d", cookie, err, len(engine.ceremonies))
	}
	engine.RevokePublic()
	if _, found := engine.ceremonies["public"]; found || len(engine.ceremonies) != 2 {
		t.Fatalf("public revoke = %#v", engine.ceremonies)
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", bytes.NewReader(nil))
	request.AddCookie(cookie)
	if _, err := engine.take(request, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.take(request, ""); !errors.Is(err, ErrCeremonyExpired) {
		t.Fatalf("reused ceremony = %v", err)
	}
	for index := 0; index < maxCeremonies; index++ {
		engine.ceremonies[string(rune(index+1))] = ceremonyState{}
	}
	if _, err := engine.start(Ceremony{CookiePath: "/"}, &webauthn.SessionData{}); !errors.Is(err, ErrCeremonyLimit) {
		t.Fatalf("ceremony limit = %v", err)
	}
	if _, err := engine.start(Ceremony{CookiePath: "/"}, nil); !errors.Is(err, ErrInvalidCeremony) {
		t.Fatalf("nil session = %v", err)
	}
	(*Engine)(nil).RevokePublic()
}

func TestCeremonyTakeRejectsMissingMalformedWrongAndExpiredCookies(t *testing.T) {
	engine := testEngine(t)
	now := time.Unix(1000, 0)
	engine.now = func() time.Time { return now }
	for _, request := range []*http.Request{
		nil,
		httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil),
		requestWithCookie(t, strings.Repeat("x", maxCookieName+1)),
	} {
		if _, err := engine.take(request, ""); err == nil {
			t.Fatal("invalid cookie was accepted")
		}
	}
	engine.ceremonies["wrong"] = ceremonyState{identity: "owner", session: webauthn.SessionData{Expires: now.Add(time.Minute)}}
	if _, err := engine.take(requestWithCookie(t, "wrong"), "other"); !errors.Is(err, ErrCeremonyExpired) {
		t.Fatalf("wrong identity = %v", err)
	}
	engine.ceremonies["expired"] = ceremonyState{session: webauthn.SessionData{Expires: now}}
	if _, err := engine.take(requestWithCookie(t, "expired"), ""); !errors.Is(err, ErrCeremonyExpired) {
		t.Fatalf("expired = %v", err)
	}
	if _, err := engine.take(requestWithCookie(t, "missing"), ""); !errors.Is(err, ErrCeremonyExpired) {
		t.Fatalf("unknown = %v", err)
	}
	if _, err := engine.take(requestWithCookie(t, "missing"), strings.Repeat("i", maxUserHandle+1)); !errors.Is(err, ErrInvalidCeremony) {
		t.Fatalf("oversized identity = %v", err)
	}
}

func TestFinishMethodsConsumeCeremoniesAndWriteNoStoreJSON(t *testing.T) { //nolint:cyclop,gocognit // All finish entry points must prove identical one-use behavior.
	engine := testEngine(t)
	user := NewUser(struct{}{}, "owner", "Owner", nil)
	_, registrationCookie, err := engine.BeginRegistration(user, Ceremony{Identity: "owner", CookiePath: "/"})
	if err != nil {
		t.Fatal(err)
	}
	registration := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{}`))
	registration.AddCookie(registrationCookie)
	if _, finishErr := engine.FinishRegistrationRequest(user, "owner", registration); finishErr == nil || errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("registration finish = %v", finishErr)
	}
	if _, finishErr := engine.FinishRegistrationRequest(user, "owner", registration); !errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("registration reuse = %v", finishErr)
	}
	_, parsedCookie, _ := engine.BeginRegistration(user, Ceremony{Identity: "owner", CookiePath: "/"})
	parsedRequest := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	parsedRequest.AddCookie(parsedCookie)
	if _, finishErr := engine.FinishRegistration(user, "owner", parsedRequest, &protocol.ParsedCredentialCreationData{}); finishErr == nil || errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("parsed registration = %v", finishErr)
	}
	_, loginCookie, _ := engine.BeginLogin(Ceremony{CookiePath: "/"})
	login := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", strings.NewReader(`{}`))
	login.AddCookie(loginCookie)
	if _, _, finishErr := engine.FinishLoginRequest(login, nil); finishErr == nil || errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("login finish = %v", finishErr)
	}
	_, parsedLoginCookie, _ := engine.BeginLogin(Ceremony{CookiePath: "/"})
	parsedLogin := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	parsedLogin.AddCookie(parsedLoginCookie)
	if _, _, finishErr := engine.FinishLogin(parsedLogin, &protocol.ParsedCredentialAssertionData{}, nil); finishErr == nil || errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("parsed login = %v", finishErr)
	}
	if _, finishErr := engine.FinishRegistrationRequest(user, "owner", httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)); !errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("missing registration = %v", finishErr)
	}
	if _, finishErr := engine.FinishRegistration(user, "owner", httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), nil); !errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("missing parsed registration = %v", finishErr)
	}
	if _, _, finishErr := engine.FinishLoginRequest(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), nil); !errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("missing login = %v", finishErr)
	}
	if _, _, finishErr := engine.FinishLogin(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), nil, nil); !errors.Is(finishErr, ErrCeremonyExpired) {
		t.Fatalf("missing parsed login = %v", finishErr)
	}
	response := httptest.NewRecorder()
	WriteJSON(response, map[string]string{"ok": "yes"})
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Content-Type") != "application/json" || response.Body.String() != "{\"ok\":\"yes\"}\n" {
		t.Fatalf("JSON response = %#v %q", response.Header(), response.Body.String())
	}
}

func requestWithCookie(t *testing.T, value string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil)
	request.AddCookie(&http.Cookie{Name: "kinosail_passkey", Value: value, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return request
}

func TestCookieValidation(t *testing.T) {
	for _, name := range []string{"cookie", "__Host-kinosail_session"} {
		if !validCookieName(name) {
			t.Fatalf("valid cookie name rejected: %q", name)
		}
	}
	for _, name := range []string{"", strings.Repeat("x", maxCookieName+1), "bad name", "bad;name", "bad\nname"} {
		if validCookieName(name) {
			t.Fatalf("invalid cookie name accepted: %q", name)
		}
	}
}
