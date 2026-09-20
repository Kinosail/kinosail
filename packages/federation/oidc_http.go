package federation

import (
	"errors"
	"net/http"
	"net/url"
)

// OIDCHTTP adapts the shared OIDC protocol to one app's profiles and sessions.
type OIDCHTTP[T any] struct {
	flow      *OIDC
	profiles  Profiles[T]
	hooks     WebHooks[T]
	renderMFA func(http.ResponseWriter, *http.Request, string) error
}

// NewOIDCHTTP creates an OIDC HTTP adapter without moving product session or audit state into the protocol package.
func NewOIDCHTTP[T any](config OIDCConfig, profiles Profiles[T], hooks WebHooks[T], renderMFA func(http.ResponseWriter, *http.Request, string) error) *OIDCHTTP[T] {
	return &OIDCHTTP[T]{flow: NewOIDC(config), profiles: profiles, hooks: hooks, renderMFA: renderMFA}
}

// Configured reports whether the OIDC flow can be registered as available.
func (login *OIDCHTTP[T]) Configured() bool { return login.flow.Configured() }

// Register adds the Player-compatible OIDC login, link, MFA, and unlink routes.
func (login *OIDCHTTP[T]) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/session/oidc", login.start)
	mux.HandleFunc("GET /api/v1/me/oidc/link", login.startLink)
	mux.HandleFunc("DELETE /api/v1/me/oidc", login.unlinkAPI)
	mux.HandleFunc("GET /login/oidc", login.start)
	mux.HandleFunc("GET /login/oidc/callback", login.callback)
	mux.HandleFunc("GET /login/mfa", login.finishMFA)
	mux.HandleFunc("POST /login/mfa", login.finishMFA)
	mux.HandleFunc("POST /account/oidc/unlink", login.unlinkWeb)
}

func (login *OIDCHTTP[T]) start(writer http.ResponseWriter, request *http.Request) {
	login.startFor(writer, request, "")
}

func (login *OIDCHTTP[T]) startLink(writer http.ResponseWriter, request *http.Request) {
	login.startFor(writer, request, login.hooks.CurrentProfileID(request))
}

func (login *OIDCHTTP[T]) startFor(writer http.ResponseWriter, request *http.Request, profileID string) {
	location, state, err := login.flow.Begin(request.Context(), profileID, login.hooks.linkSession(request, profileID))
	if err != nil {
		if errors.Is(err, ErrTooManyPending) {
			login.hooks.Error(writer, request, "too many SSO requests are pending", http.StatusTooManyRequests)
			return
		}
		login.hooks.Error(writer, request, "SSO is unavailable", http.StatusServiceUnavailable)
		return
	}
	http.SetCookie(writer, StateCookie(state, login.hooks.SecureRequest(request)))
	http.Redirect(writer, request, location, http.StatusSeeOther)
}

func (login *OIDCHTTP[T]) callback(writer http.ResponseWriter, request *http.Request) {
	stateCookie, cookieErr := request.Cookie(OIDCStateCookieName)
	cookieState := ""
	if cookieErr == nil {
		cookieState = stateCookie.Value
	}
	result, err := login.flow.Complete(request.Context(), request.URL.RawQuery, cookieState, cookieErr == nil)
	if result.ClearCookie {
		http.SetCookie(writer, StateCookie("", login.hooks.SecureRequest(request)))
	}
	if err != nil {
		login.writeCallbackError(writer, request, err)
		return
	}
	login.acceptCallback(writer, request, result)
}

func (login *OIDCHTTP[T]) acceptCallback(writer http.ResponseWriter, request *http.Request, result OIDCCallback) {
	if result.ProfileID != "" {
		if err := login.profiles.LinkForSession(OIDCProtocol, result.ProfileID, result.Identity, result.LinkSession); err != nil {
			login.hooks.Error(writer, request, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(writer, request, "/account", http.StatusSeeOther)
		return
	}
	profile, found := login.profiles.Find(OIDCProtocol, result.Identity)
	var err error
	if !found {
		profile, found, err = login.profiles.AutoLinkSCIM(OIDCProtocol, result.Identity)
	}
	if err != nil {
		login.hooks.Error(writer, request, "SSO session could not be created", http.StatusInternalServerError)
		return
	}
	if !found {
		login.hooks.Error(writer, request, "SSO identity is not linked to a Viewer Profile", http.StatusUnauthorized)
		return
	}
	login.SignInProfile(writer, request, profile)
}

// SignInProfile completes app-local MFA and session work for a verified federated profile.
func (login *OIDCHTTP[T]) SignInProfile(writer http.ResponseWriter, request *http.Request, profile T) {
	if login.hooks.RequiresMFA(profile) {
		challenge, err := login.BeginMFA(login.hooks.ProfileID(profile))
		if err != nil {
			login.hooks.Error(writer, request, "too many SSO requests are pending", http.StatusTooManyRequests)
			return
		}
		http.Redirect(writer, request, "/login/mfa?challenge="+url.QueryEscape(challenge), http.StatusSeeOther)
		return
	}
	if login.hooks.signIn(writer, request, profile) {
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	}
}

// BeginMFA creates a challenge reusable by the SAML HTTP adapter.
func (login *OIDCHTTP[T]) BeginMFA(profileID string) (string, error) {
	return login.flow.BeginMFA(profileID)
}

func (login *OIDCHTTP[T]) finishMFA(writer http.ResponseWriter, request *http.Request) {
	challenge := request.FormValue("challenge")
	profileID, found := login.flow.MFAProfile(challenge)
	if request.Method == http.MethodGet {
		if !found {
			login.hooks.Error(writer, request, "SSO challenge is invalid or expired", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = login.renderMFA(writer, request, challenge)
		return
	}
	if !found || !login.hooks.VerifySecondFactor(profileID, request.FormValue("code")) {
		login.hooks.Error(writer, request, "invalid authentication code", http.StatusUnauthorized)
		return
	}
	consumedProfile, consumed := login.flow.ConsumeMFA(challenge)
	if !consumed || consumedProfile != profileID {
		login.hooks.Error(writer, request, "invalid authentication code", http.StatusUnauthorized)
		return
	}
	profile, found := login.profiles.FindByID(profileID)
	if !found {
		login.hooks.Error(writer, request, "SSO session could not be created", http.StatusInternalServerError)
		return
	}
	if !login.hooks.signIn(writer, request, profile) {
		return
	}
	http.Redirect(writer, request, "/", http.StatusSeeOther)
}

func (login *OIDCHTTP[T]) unlinkAPI(writer http.ResponseWriter, request *http.Request) {
	if err := login.profiles.Unlink(OIDCProtocol, login.hooks.CurrentProfileID(request)); err != nil {
		login.hooks.APIError(writer, err, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (login *OIDCHTTP[T]) unlinkWeb(writer http.ResponseWriter, request *http.Request) {
	if err := login.profiles.Unlink(OIDCProtocol, login.hooks.CurrentProfileID(request)); err != nil {
		login.hooks.Error(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, "/account", http.StatusSeeOther)
}

func (login *OIDCHTTP[T]) writeCallbackError(writer http.ResponseWriter, request *http.Request, err error) {
	message, status := "SSO callback is invalid", http.StatusBadRequest
	switch {
	case errors.Is(err, ErrInvalidState):
		message = "SSO state is invalid or expired"
	case errors.Is(err, ErrAuthorizationDenied):
		message = "SSO authorization was denied"
	case errors.Is(err, ErrProviderUnavailable):
		message, status = "SSO provider is unavailable", http.StatusBadGateway
	case errors.Is(err, ErrCodeExchange):
		message, status = "SSO code exchange failed", http.StatusBadGateway
	case errors.Is(err, ErrTokenInvalid):
		message, status = "SSO token validation failed", http.StatusUnauthorized
	}
	login.hooks.Error(writer, request, message, status)
}
