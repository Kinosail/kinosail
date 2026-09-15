package federation

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/crewjam/saml"
)

func TestOIDCHTTPCompletesLoginAndLinkCallbacks(t *testing.T) { //nolint:funlen // The provider lifecycle proves both callback destinations.
	flow, _, setNonce := newOIDCTestFlow(t)
	identity := Identity{Issuer: flow.config.Issuer, Subject: "subject-1"}
	values := []webProfile{{ID: "viewer", OIDC: identity}}
	effects := &webEffects{}
	login := &OIDCHTTP[webProfile]{flow: flow, profiles: webProfiles(&values), hooks: webHooks(effects), renderMFA: func(http.ResponseWriter, *http.Request, string) error { return nil }}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/oidc", nil)
	response := httptest.NewRecorder()
	login.start(response, request)
	completeOIDCHTTPCallback(t, login, response, setNonce)
	if effects.signIns != 1 || effects.audits != 1 {
		t.Fatalf("OIDC login effects = %#v", effects)
	}

	values[0].OIDC = Identity{}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me/oidc/link", nil)
	response = httptest.NewRecorder()
	login.startLink(response, request)
	completeOIDCHTTPCallback(t, login, response, setNonce)
	if values[0].OIDC != identity {
		t.Fatalf("OIDC link = %#v", values[0].OIDC)
	}
}

func completeOIDCHTTPCallback(t *testing.T, login *OIDCHTTP[webProfile], begin *httptest.ResponseRecorder, setNonce func(string)) {
	t.Helper()
	location, err := url.Parse(begin.Header().Get("Location"))
	cookies := begin.Result().Cookies()
	if err != nil || begin.Code != http.StatusSeeOther || len(cookies) != 1 {
		t.Fatalf("OIDC start = %d %q %#v %v", begin.Code, location, cookies, err)
	}
	setNonce(location.Query().Get("nonce"))
	callback := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/oidc/callback?code=approved&state="+url.QueryEscape(location.Query().Get("state")), nil)
	callback.AddCookie(cookies[0])
	response := httptest.NewRecorder()
	login.callback(response, callback)
	if response.Code != http.StatusSeeOther || len(response.Result().Cookies()) != 1 || response.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("OIDC callback = %d %#v %q", response.Code, response.Result().Cookies(), response.Body.String())
	}
}

func TestOIDCHTTPRejectsCapacityPersistenceAndMFAEdges(t *testing.T) { //nolint:funlen // Each edge maps a bounded failure to its public response.
	flow, _, _ := newOIDCTestFlow(t)
	if _, err := flow.getProvider(t.Context()); err != nil {
		t.Fatal(err)
	}
	values := []webProfile{{ID: "viewer", MFA: true}}
	effects := &webEffects{}
	login := &OIDCHTTP[webProfile]{flow: flow, profiles: webProfilesWithPersist(&values, errors.New("persist failed")), hooks: webHooks(effects), renderMFA: func(http.ResponseWriter, *http.Request, string) error { return nil }}
	for index := range maxPendingStates {
		flow.pending[string(rune(index+1))] = oidcTransaction{expires: time.Now().Add(time.Minute)}
	}
	response := httptest.NewRecorder()
	login.startFor(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), "viewer")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("OIDC capacity response = %d", response.Code)
	}
	flow.pending = make(map[string]oidcTransaction)
	login.acceptCallback(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), OIDCCallback{ProfileID: "viewer", Identity: Identity{Issuer: "issuer", Subject: "subject"}})

	for index := range maxPendingStates {
		flow.mfa[string(rune(index+1))] = mfaChallenge{expires: time.Now().Add(time.Minute)}
	}
	login.SignInProfile(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), values[0])
	flow.mfa = make(map[string]mfaChallenge)
	assertHTTPStatus(t, http.HandlerFunc(login.finishMFA), http.MethodGet, "/login/mfa?challenge=missing", "", http.StatusBadRequest)

	challenge, _ := flow.BeginMFA("missing")
	assertHTTPStatus(t, http.HandlerFunc(login.finishMFA), http.MethodPost, "/login/mfa", "challenge="+challenge+"&code=valid", http.StatusInternalServerError)
	values[0].MFA = false
	effects.signInErr = errors.New("session failed")
	challenge, _ = flow.BeginMFA("viewer")
	assertHTTPStatus(t, http.HandlerFunc(login.finishMFA), http.MethodPost, "/login/mfa", "challenge="+challenge+"&code=valid", http.StatusInternalServerError)
}

func TestOIDCHTTPRejectsConsumedMFAAndUnlinkPersistence(t *testing.T) {
	values := []webProfile{{ID: "viewer"}}
	effects := &webEffects{}
	login := NewOIDCHTTP(OIDCConfig{}, webProfilesWithPersist(&values, errors.New("persist failed")), webHooks(effects), func(http.ResponseWriter, *http.Request, string) error { return nil })
	challenge, _ := login.BeginMFA("viewer")
	login.hooks.VerifySecondFactor = func(string, string) bool {
		_, _ = login.flow.ConsumeMFA(challenge)
		return true
	}
	assertHTTPStatus(t, http.HandlerFunc(login.finishMFA), http.MethodPost, "/login/mfa", "challenge="+challenge+"&code=valid", http.StatusUnauthorized)
	assertHTTPStatus(t, http.HandlerFunc(login.unlinkAPI), http.MethodDelete, "/api/v1/me/oidc", "", http.StatusBadRequest)
	assertHTTPStatus(t, http.HandlerFunc(login.unlinkWeb), http.MethodPost, "/account/oidc/unlink", "", http.StatusBadRequest)
}

func TestOIDCHTTPAutoLinkPersistenceFailure(t *testing.T) {
	values := []webProfile{{ID: "viewer", Managed: true, External: "directory"}}
	effects := &webEffects{}
	login := NewOIDCHTTP(OIDCConfig{}, webProfilesWithPersist(&values, errors.New("persist failed")), webHooks(effects), func(http.ResponseWriter, *http.Request, string) error { return nil })
	response := httptest.NewRecorder()
	login.acceptCallback(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), OIDCCallback{Identity: Identity{Issuer: "issuer", Subject: "directory"}})
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("OIDC auto-link persistence = %d", response.Code)
	}
}

func TestSAMLHTTPStartsMetadataAndMFA(t *testing.T) { //nolint:funlen // Redirect, post, metadata, and MFA adapters share one configuration fixture.
	certificate := samlMetadataCertificate(t)
	identity := Identity{Issuer: "issuer", Subject: "subject"}
	values := []webProfile{{ID: "viewer", SAML: identity, MFA: true}}
	effects := &webEffects{}
	oidc := NewOIDCHTTP(OIDCConfig{}, webProfiles(&values), webHooks(effects), func(http.ResponseWriter, *http.Request, string) error { return nil })
	config := SAMLConfig{MetadataXML: samlMetadataDocument("https://identity.example", "/sso", certificate), RootURL: "http://localhost", DataDir: t.TempDir()}
	login := NewSAMLHTTP(config, webProfiles(&values), webHooks(effects), oidc)

	response := httptest.NewRecorder()
	login.start(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/saml", nil))
	if response.Code != http.StatusSeeOther || response.Header().Get("Location") == "" {
		t.Fatalf("SAML redirect start = %d %q", response.Code, response.Header().Get("Location"))
	}
	response = httptest.NewRecorder()
	login.startLink(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/me/saml/link", nil))
	if response.Code != http.StatusSeeOther {
		t.Fatalf("SAML link start = %d", response.Code)
	}

	postConfig := config
	postConfig.MetadataXML = strings.Replace(config.MetadataXML, saml.HTTPRedirectBinding, saml.HTTPPostBinding, 1)
	postLogin := NewSAMLHTTP(postConfig, webProfiles(&values), webHooks(effects), oidc)
	response = httptest.NewRecorder()
	postLogin.start(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/saml", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Security-Policy") == "" || !strings.Contains(response.Body.String(), "SAMLRequest") {
		t.Fatalf("SAML post start = %d %q", response.Code, response.Body.String())
	}

	metadataLogin := NewSAMLHTTP(SAMLConfig{RootURL: "http://localhost", DataDir: t.TempDir()}, webProfiles(&values), webHooks(effects))
	response = httptest.NewRecorder()
	metadataLogin.metadata(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/login/saml/metadata", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/samlmetadata+xml" {
		t.Fatalf("SAML metadata = %d %q", response.Code, response.Body.String())
	}
	login.signInProfile(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), values[0])
	if len(oidc.flow.mfa) != 1 {
		t.Fatal("SAML MFA challenge was not created")
	}
}

func TestSAMLHTTPRejectsCapacityProfileAndStorageEdges(t *testing.T) { //nolint:funlen // Error adapters are verified independently of signed-response parsing.
	certificate := samlMetadataCertificate(t)
	config := SAMLConfig{MetadataXML: samlMetadataDocument("https://identity.example", "/sso", certificate), RootURL: "http://localhost", DataDir: t.TempDir()}
	values := []webProfile{{ID: "viewer", Managed: true, External: "directory", MFA: true}}
	effects := &webEffects{}
	profiles := webProfilesWithPersist(&values, errors.New("persist failed"))
	oidc := NewOIDCHTTP(OIDCConfig{}, profiles, webHooks(effects), func(http.ResponseWriter, *http.Request, string) error { return nil })
	login := NewSAMLHTTP(config, profiles, webHooks(effects), oidc)
	for index := range maxPendingStates {
		login.flow.pending[string(rune(index+1))] = samlTransaction{expires: time.Now().Add(time.Minute)}
		oidc.flow.mfa[string(rune(index+1))] = mfaChallenge{expires: time.Now().Add(time.Minute)}
	}
	response := httptest.NewRecorder()
	login.startFor(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), "viewer")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("SAML capacity response = %d", response.Code)
	}
	login.signInIdentity(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), Identity{Issuer: "issuer", Subject: "directory"})
	login.signInIdentity(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), Identity{Issuer: "issuer", Subject: "missing"})
	login.signInProfile(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), values[0])
	login.acceptCallback(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), SAMLCallback{ProfileID: "viewer", Identity: Identity{Issuer: "issuer", Subject: "linked"}})
	assertHTTPStatus(t, http.HandlerFunc(login.unlinkAPI), http.MethodDelete, "/api/v1/me/saml", "", http.StatusBadRequest)
	assertHTTPStatus(t, http.HandlerFunc(login.unlinkWeb), http.MethodPost, "/account/saml/unlink", "", http.StatusBadRequest)
	values[0].Managed, values[0].MFA = false, false
	effects.signInErr = errors.New("session failed")
	login.signInProfile(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil), values[0])

	unavailable := NewSAMLHTTP(SAMLConfig{}, profiles, webHooks(effects))
	response = httptest.NewRecorder()
	unavailable.metadata(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unavailable SAML metadata = %d", response.Code)
	}

	successValues := []webProfile{{ID: "viewer"}}
	success := NewSAMLHTTP(config, webProfiles(&successValues), webHooks(&webEffects{}))
	want := Identity{Issuer: "issuer", Subject: "linked"}
	response = httptest.NewRecorder()
	success.acceptCallback(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), SAMLCallback{ProfileID: "viewer", Identity: want})
	if response.Code != http.StatusSeeOther || successValues[0].SAML != want {
		t.Fatalf("SAML link callback = %d %#v", response.Code, successValues[0].SAML)
	}
	response = httptest.NewRecorder()
	success.acceptCallback(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil), SAMLCallback{Identity: want})
	if response.Code != http.StatusSeeOther {
		t.Fatalf("SAML sign-in callback = %d", response.Code)
	}
}

func webProfilesWithPersist(values *[]webProfile, persistErr error) Profiles[webProfile] {
	var mutex sync.Mutex
	profiles := webProfiles(values)
	profiles.Lock, profiles.Unlock = mutex.Lock, mutex.Unlock
	profiles.Persist = func([]webProfile) error { return persistErr }
	return profiles
}
