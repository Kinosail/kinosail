package passkeys

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

func TestProfileFlowFinishesRegistrationOutcomes(t *testing.T) {
	fixture, flow := newFlowFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/register/finish", nil)
	credential := &webauthn.Credential{}
	flow.finishRegistration = func(webauthn.User, string, *http.Request) (*webauthn.Credential, error) {
		return credential, errors.New("invalid response")
	}
	flow.FinishRegistration(httptest.NewRecorder(), request)
	if fixture.statuses[len(fixture.statuses)-1] != http.StatusBadRequest || fixture.adds != 0 {
		t.Fatal("invalid response caused registration effects")
	}
	flow.finishRegistration = func(webauthn.User, string, *http.Request) (*webauthn.Credential, error) { return credential, nil }
	flow.config.AddCredential = func(string, *webauthn.Credential) error { return errors.New("save failed") }
	flow.FinishRegistration(httptest.NewRecorder(), request)
	flow.config.AddCredential = func(string, *webauthn.Credential) error { return nil }
	flow.config.MarkStrong = func(*http.Request) error { return errors.New("session failed") }
	response := httptest.NewRecorder()
	flow.FinishRegistration(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("session failure = %d", response.Code)
	}
	flow.config.MarkStrong = func(*http.Request) error { return nil }
	response = httptest.NewRecorder()
	flow.FinishRegistration(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("registration = %d", response.Code)
	}
}

func TestProfileFlowFinishesLoginOutcomes(t *testing.T) { //nolint:cyclop // One table-like sequence covers the ordered login effects.
	fixture, flow := newFlowFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/login/finish", nil)
	credential := &webauthn.Credential{}
	setResult := func(user webauthn.User, err error) {
		flow.finishLogin = func(*http.Request, webauthn.DiscoverableUserHandler) (webauthn.User, *webauthn.Credential, error) {
			return user, credential, err
		}
	}
	setResult(nil, errors.New("invalid response"))
	flow.FinishLogin(httptest.NewRecorder(), request)
	setResult(NewUser(flowProfile{ID: "owner", Owner: true, Remote: true}, "owner", "Owner", nil), nil)
	fixture.public = true
	response := httptest.NewRecorder()
	flow.FinishLogin(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("public owner = %d", response.Code)
	}
	setResult(NewUser(flowProfile{ID: "viewer"}, "viewer", "Viewer", nil), nil)
	flow.FinishLogin(httptest.NewRecorder(), request)
	setResult(NewUser(flowProfile{ID: "viewer", Remote: true}, "viewer", "Viewer", nil), nil)
	flow.config.UpdateCredential = func(string, *webauthn.Credential) error { return errors.New("save failed") }
	flow.FinishLogin(httptest.NewRecorder(), request)
	flow.config.UpdateCredential = func(string, *webauthn.Credential) error { return nil }
	flow.config.SignIn = func(http.ResponseWriter, *http.Request, string, bool) error { return errors.New("sign in failed") }
	flow.FinishLogin(httptest.NewRecorder(), request)
	flow.config.SignIn = func(http.ResponseWriter, *http.Request, string, bool) error { fixture.signIns++; return nil }
	fixture.onboarding = true
	response = httptest.NewRecorder()
	flow.FinishLogin(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("X-Kinosail-Login-Next") != "/onboarding" || fixture.audits != 1 {
		t.Fatalf("login = %d %#v, audits %d", response.Code, response.Header(), fixture.audits)
	}
}
