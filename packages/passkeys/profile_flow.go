package passkeys

import (
	"errors"
	"net/http"

	"github.com/go-webauthn/webauthn/webauthn"
)

// ProfileFlowConfig connects Player-style account behavior to the shared protocol engine.
type ProfileFlowConfig[T any] struct {
	Engine           *Engine
	Current          func(*http.Request) T
	Identity         func(T) (string, string, []webauthn.Credential)
	Owner            func(T) bool
	Remote           func(T) bool
	PublicRequest    func(*http.Request) bool
	SecureRequest    func(*http.Request) bool
	AddCredential    func(string, *webauthn.Credential) error
	MarkStrong       func(*http.Request) error
	Discover         webauthn.DiscoverableUserHandler
	UpdateCredential func(string, *webauthn.Credential) error
	SignIn           func(http.ResponseWriter, *http.Request, string, bool) error
	AfterLogin       func(*http.Request, T, *webauthn.Credential)
	OnboardingNext   func(T) bool
	WriteError       func(http.ResponseWriter, error, int)
}

// ProfileFlow owns Player-style registration and login orchestration.
type ProfileFlow[T any] struct {
	config             ProfileFlowConfig[T]
	finishRegistration func(webauthn.User, string, *http.Request) (*webauthn.Credential, error)
	finishLogin        func(*http.Request, webauthn.DiscoverableUserHandler) (webauthn.User, *webauthn.Credential, error)
}

// NewProfileFlow validates and creates one Player-style passkey flow.
func NewProfileFlow[T any](config ProfileFlowConfig[T]) (*ProfileFlow[T], error) {
	if !validProfileFlow(config) {
		return nil, ErrInvalidConfig
	}
	return &ProfileFlow[T]{config: config, finishRegistration: config.Engine.FinishRegistrationRequest, finishLogin: config.Engine.FinishLoginRequest}, nil
}

func validProfileFlow[T any](config ProfileFlowConfig[T]) bool {
	return validProfileFlowIdentity(config) && validProfileFlowEffects(config)
}

func validProfileFlowIdentity[T any](config ProfileFlowConfig[T]) bool {
	return config.Engine != nil && config.Current != nil && config.Identity != nil && config.Owner != nil && config.Remote != nil && config.PublicRequest != nil && config.SecureRequest != nil && config.Discover != nil
}

func validProfileFlowEffects[T any](config ProfileFlowConfig[T]) bool {
	return config.AddCredential != nil && config.MarkStrong != nil && config.UpdateCredential != nil && config.SignIn != nil && config.AfterLogin != nil && config.OnboardingNext != nil && config.WriteError != nil
}

// BeginRegistration starts one authenticated registration ceremony.
func (flow *ProfileFlow[T]) BeginRegistration(writer http.ResponseWriter, request *http.Request) {
	if flow == nil || !flow.requireOrigin(writer, request, "/account") {
		return
	}
	profile := flow.config.Current(request)
	id, name, credentials := flow.config.Identity(profile)
	creation, cookie, err := flow.config.Engine.BeginRegistration(NewUser(profile, id, name, credentials), flow.ceremony(request, id))
	if err != nil {
		flow.config.WriteError(writer, errors.New("passkey registration is unavailable"), http.StatusServiceUnavailable)
		return
	}
	http.SetCookie(writer, cookie) //nolint:gosec // Plaintext localhost must complete a passkey ceremony.
	WriteJSON(writer, creation)
}

// FinishRegistration verifies and commits one credential.
func (flow *ProfileFlow[T]) FinishRegistration(writer http.ResponseWriter, request *http.Request) {
	profile := flow.config.Current(request)
	id, name, credentials := flow.config.Identity(profile)
	credential, err := flow.finishRegistration(NewUser(profile, id, name, credentials), id, request)
	if errors.Is(err, ErrCeremonyExpired) {
		flow.config.WriteError(writer, errors.New("passkey ceremony expired"), http.StatusBadRequest)
		return
	}
	if err != nil || flow.config.AddCredential(id, credential) != nil {
		flow.config.WriteError(writer, errors.New("passkey registration failed"), http.StatusBadRequest)
		return
	}
	if err := flow.config.MarkStrong(request); err != nil {
		flow.config.WriteError(writer, errors.New("could not secure current session"), http.StatusInternalServerError)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

// BeginLogin starts one discoverable login ceremony.
func (flow *ProfileFlow[T]) BeginLogin(writer http.ResponseWriter, request *http.Request) {
	if flow == nil || !flow.requireOrigin(writer, request, "/login") {
		return
	}
	assertion, cookie, err := flow.config.Engine.BeginLogin(flow.ceremony(request, ""))
	if err != nil {
		flow.config.WriteError(writer, errors.New("passkey login is unavailable"), http.StatusServiceUnavailable)
		return
	}
	http.SetCookie(writer, cookie) //nolint:gosec // Plaintext localhost must complete a passkey ceremony.
	WriteJSON(writer, assertion)
}

// FinishLogin verifies and commits one discoverable login.
func (flow *ProfileFlow[T]) FinishLogin(writer http.ResponseWriter, request *http.Request) {
	user, credential, err := flow.finishLogin(request, flow.config.Discover)
	if errors.Is(err, ErrCeremonyExpired) {
		flow.config.WriteError(writer, errors.New("passkey ceremony expired"), http.StatusBadRequest)
		return
	}
	found, valid := user.(User[T])
	profile := found.Value
	public := flow.config.PublicRequest(request)
	if valid && flow.publicLoginDenied(profile, public) {
		flow.config.WriteError(writer, errors.New("passkey login is not permitted for this public profile"), http.StatusForbidden)
		return
	}
	id, _, _ := flow.config.Identity(profile)
	if err != nil || !valid || flow.config.UpdateCredential(id, credential) != nil || flow.config.SignIn(writer, request, id, public) != nil {
		flow.config.WriteError(writer, errors.New("passkey login failed"), http.StatusUnauthorized)
		return
	}
	flow.config.AfterLogin(request, profile, credential)
	if flow.config.OnboardingNext(profile) {
		writer.Header().Set("X-Kinosail-Login-Next", "/onboarding")
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (flow *ProfileFlow[T]) publicLoginDenied(profile T, public bool) bool {
	return public && (flow.config.Owner(profile) || !flow.config.Remote(profile))
}

func (flow *ProfileFlow[T]) requireOrigin(writer http.ResponseWriter, request *http.Request, path string) bool {
	if flow.config.Engine.Origin().Matches(request, flow.config.SecureRequest(request)) {
		return true
	}
	writer.Header().Set("Location", flow.config.Engine.Origin().String()+path)
	flow.config.WriteError(writer, errors.New("passkeys require the configured authentication origin"), http.StatusMisdirectedRequest)
	return false
}

func (flow *ProfileFlow[T]) ceremony(request *http.Request, identity string) Ceremony {
	path := "/api/v1/passkeys/"
	if len(request.URL.Path) >= len("/auth/") && request.URL.Path[:len("/auth/")] == "/auth/" {
		path = "/auth/passkeys/"
	}
	return Ceremony{Identity: identity, CookiePath: path, Secure: flow.config.SecureRequest(request), Public: flow.config.PublicRequest(request)}
}
