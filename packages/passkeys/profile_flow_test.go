package passkeys

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-webauthn/webauthn/webauthn"
)

type flowProfile struct {
	ID          string
	Name        string
	Owner       bool
	Remote      bool
	Credentials []webauthn.Credential
}

type flowFixture struct {
	engine     *Engine
	profile    flowProfile
	errors     []error
	statuses   []int
	adds       int
	strong     int
	updates    int
	signIns    int
	audits     int
	public     bool
	secure     bool
	onboarding bool
}

func newFlowFixture(t *testing.T) (*flowFixture, *ProfileFlow[flowProfile]) {
	t.Helper()
	fixture := &flowFixture{engine: testEngine(t), profile: flowProfile{ID: "profile", Name: "Viewer"}}
	flow, err := NewProfileFlow(ProfileFlowConfig[flowProfile]{
		Engine:  fixture.engine,
		Current: func(*http.Request) flowProfile { return fixture.profile },
		Identity: func(profile flowProfile) (string, string, []webauthn.Credential) {
			return profile.ID, profile.Name, profile.Credentials
		},
		Owner: func(profile flowProfile) bool { return profile.Owner }, Remote: func(profile flowProfile) bool { return profile.Remote },
		PublicRequest: func(*http.Request) bool { return fixture.public }, SecureRequest: func(*http.Request) bool { return fixture.secure },
		AddCredential: func(string, *webauthn.Credential) error { fixture.adds++; return nil }, MarkStrong: func(*http.Request) error { fixture.strong++; return nil },
		Discover:         func(_, _ []byte) (webauthn.User, error) { return nil, ErrNotFound },
		UpdateCredential: func(string, *webauthn.Credential) error { fixture.updates++; return nil },
		SignIn:           func(http.ResponseWriter, *http.Request, string, bool) error { fixture.signIns++; return nil },
		AfterLogin:       func(*http.Request, flowProfile, *webauthn.Credential) { fixture.audits++ }, OnboardingNext: func(flowProfile) bool { return fixture.onboarding },
		WriteError: func(writer http.ResponseWriter, err error, status int) {
			fixture.errors = append(fixture.errors, err)
			fixture.statuses = append(fixture.statuses, status)
			http.Error(writer, err.Error(), status)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return fixture, flow
}

func TestProfileFlowRequiresCompleteConfiguration(t *testing.T) {
	if _, err := NewProfileFlow(ProfileFlowConfig[flowProfile]{}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("empty config = %v", err)
	}
	fixture, flow := newFlowFixture(t)
	if fixture == nil || flow == nil {
		t.Fatal("valid flow was not created")
	}
}

func TestProfileFlowBeginsRegistrationAndLoginWithBoundCeremonies(t *testing.T) {
	fixture, flow := newFlowFixture(t)
	registration := httptest.NewRecorder()
	flow.BeginRegistration(registration, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/register/begin", nil))
	if registration.Code != http.StatusOK || registration.Header().Get("Cache-Control") != "no-store" || len(registration.Result().Cookies()) != 1 || registration.Result().Cookies()[0].Path != "/api/v1/passkeys/" || len(fixture.engine.ceremonies) != 1 {
		t.Fatalf("registration = %d %#v, ceremonies %d", registration.Code, registration.Header(), len(fixture.engine.ceremonies))
	}
	fixture.public = true
	login := httptest.NewRecorder()
	flow.BeginLogin(login, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/auth/passkeys/login/begin", nil))
	loginCookie := login.Result().Cookies()[0]
	if login.Code != http.StatusOK || loginCookie.Path != "/auth/passkeys/" || !fixture.engine.ceremonies[loginCookie.Value].public {
		t.Fatalf("login = %d %#v", login.Code, loginCookie)
	}
}

func TestProfileFlowRejectsWrongOriginBeforeStarting(t *testing.T) {
	fixture, flow := newFlowFixture(t)
	for _, begin := range []func(http.ResponseWriter, *http.Request){flow.BeginRegistration, flow.BeginLogin} {
		response := httptest.NewRecorder()
		begin(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://outside.example/auth/passkeys/login/begin", nil))
		if response.Code != http.StatusMisdirectedRequest || response.Header().Get("Location") == "" {
			t.Fatalf("wrong origin = %d %#v", response.Code, response.Header())
		}
	}
	if len(fixture.engine.ceremonies) != 0 || len(fixture.errors) != 2 {
		t.Fatal("wrong origin started a ceremony")
	}
	var nilFlow *ProfileFlow[flowProfile]
	nilFlow.BeginRegistration(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/", nil))
	nilFlow.BeginLogin(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/", nil))
}

func TestProfileFlowMapsUnavailableAndExpiredWithoutAccountEffects(t *testing.T) { //nolint:cyclop // One scenario proves all account side effects stay absent.
	fixture, flow := newFlowFixture(t)
	for index := 0; index < maxCeremonies; index++ {
		fixture.engine.ceremonies[string(rune(index+1))] = ceremonyState{}
	}
	response := httptest.NewRecorder()
	flow.BeginRegistration(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/register/begin", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("ceremony limit = %d", response.Code)
	}
	fixture.engine.ceremonies = make(map[string]ceremonyState)
	for index := 0; index < maxCeremonies; index++ {
		fixture.engine.ceremonies[string(rune(index+1))] = ceremonyState{}
	}
	flow.BeginLogin(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/login/begin", nil))
	fixture.engine.ceremonies = make(map[string]ceremonyState)

	for _, finish := range []func(http.ResponseWriter, *http.Request){flow.FinishRegistration, flow.FinishLogin} {
		response = httptest.NewRecorder()
		finish(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "http://localhost:38127/api/v1/passkeys/finish", nil))
		if response.Code != http.StatusBadRequest || fixture.statuses[len(fixture.statuses)-1] != http.StatusBadRequest {
			t.Fatalf("expired ceremony = %d %#v", response.Code, fixture.statuses)
		}
	}
	if fixture.adds != 0 || fixture.strong != 0 || fixture.updates != 0 || fixture.signIns != 0 || fixture.audits != 0 {
		t.Fatalf("rejected ceremonies caused account effects: %#v", fixture)
	}
}
