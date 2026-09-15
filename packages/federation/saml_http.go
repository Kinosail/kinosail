package federation

import (
	"errors"
	"net/http"
	"net/url"
)

// SAMLHTTP adapts the shared SAML protocol to one app's profiles and sessions.
type SAMLHTTP[T any] struct {
	flow     *SAML
	profiles Profiles[T]
	hooks    WebHooks[T]
	mfa      *OIDCHTTP[T]
}

// NewSAMLHTTP creates a SAML HTTP adapter and optionally reuses OIDC's bounded MFA challenges.
func NewSAMLHTTP[T any](config SAMLConfig, profiles Profiles[T], hooks WebHooks[T], mfa ...*OIDCHTTP[T]) *SAMLHTTP[T] {
	login := &SAMLHTTP[T]{flow: NewSAML(config), profiles: profiles, hooks: hooks}
	if len(mfa) != 0 {
		login.mfa = mfa[0]
	}
	return login
}

// Configured reports whether the SAML flow can be registered as available.
func (login *SAMLHTTP[T]) Configured() bool { return login.flow.Configured() }

// Register adds the Player-compatible SAML metadata, login, link, and unlink routes.
func (login *SAMLHTTP[T]) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/session/saml", login.start)
	mux.HandleFunc("GET /api/v1/me/saml/link", login.startLink)
	mux.HandleFunc("DELETE /api/v1/me/saml", login.unlinkAPI)
	mux.HandleFunc("GET /login/saml", login.start)
	mux.HandleFunc("GET /login/saml/metadata", login.metadata)
	mux.HandleFunc("POST /login/saml/acs", login.callback)
	mux.HandleFunc("POST /account/saml/unlink", login.unlinkWeb)
}

func (login *SAMLHTTP[T]) callback(writer http.ResponseWriter, request *http.Request) {
	result, err := login.flow.Complete(writer, request)
	if err != nil {
		login.writeCallbackError(writer, request, err)
		return
	}
	login.acceptCallback(writer, request, result)
}

func (login *SAMLHTTP[T]) acceptCallback(writer http.ResponseWriter, request *http.Request, result SAMLCallback) {
	if result.ProfileID != "" {
		if err := login.profiles.Link(SAMLProtocol, result.ProfileID, result.Identity); err != nil {
			login.hooks.Error(writer, request, err.Error(), http.StatusConflict)
			return
		}
		http.Redirect(writer, request, "/account", http.StatusSeeOther)
		return
	}
	login.signInIdentity(writer, request, result.Identity)
}

func (login *SAMLHTTP[T]) signInIdentity(writer http.ResponseWriter, request *http.Request, identity Identity) {
	profile, found := login.profiles.Find(SAMLProtocol, identity)
	var err error
	if !found {
		profile, found, err = login.profiles.AutoLinkSCIM(SAMLProtocol, identity)
	}
	if err != nil {
		login.hooks.Error(writer, request, "SSO session could not be created", http.StatusInternalServerError)
		return
	}
	if !found {
		login.hooks.Error(writer, request, "SSO identity is not linked to a Viewer Profile", http.StatusUnauthorized)
		return
	}
	login.signInProfile(writer, request, profile)
}

func (login *SAMLHTTP[T]) signInProfile(writer http.ResponseWriter, request *http.Request, profile T) {
	if login.hooks.RequiresMFA(profile) {
		if login.mfa == nil {
			login.hooks.Error(writer, request, "SSO session could not be created", http.StatusServiceUnavailable)
			return
		}
		challenge, err := login.mfa.BeginMFA(login.hooks.ProfileID(profile))
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

func (login *SAMLHTTP[T]) writeCallbackError(writer http.ResponseWriter, request *http.Request, err error) {
	message, status := "SAML callback is invalid", http.StatusBadRequest
	switch {
	case errors.Is(err, ErrInvalidState):
		message = "SAML response is invalid or expired"
	case errors.Is(err, ErrProviderUnavailable), errors.Is(err, ErrNotConfigured):
		message, status = "SAML provider is unavailable", http.StatusBadGateway
	case errors.Is(err, ErrTokenInvalid):
		message, status = "SAML response validation failed", http.StatusUnauthorized
	}
	login.hooks.Error(writer, request, message, status)
}

func (login *SAMLHTTP[T]) metadata(writer http.ResponseWriter, request *http.Request) {
	data, err := login.flow.Metadata(request.Context())
	if err != nil {
		login.hooks.Error(writer, request, "SAML metadata is unavailable", http.StatusServiceUnavailable)
		return
	}
	writer.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = writer.Write(data)
}

func (login *SAMLHTTP[T]) start(writer http.ResponseWriter, request *http.Request) {
	login.startFor(writer, request, "")
}

func (login *SAMLHTTP[T]) startLink(writer http.ResponseWriter, request *http.Request) {
	login.startFor(writer, request, login.hooks.CurrentProfileID(request))
}

func (login *SAMLHTTP[T]) startFor(writer http.ResponseWriter, request *http.Request, profileID string) {
	start, err := login.flow.Begin(request.Context(), profileID)
	if err != nil {
		if errors.Is(err, ErrTooManyPending) {
			login.hooks.Error(writer, request, "too many SSO requests are pending", http.StatusTooManyRequests)
			return
		}
		login.hooks.Error(writer, request, "SAML SSO is unavailable", http.StatusServiceUnavailable)
		return
	}
	if len(start.PostHTML) != 0 {
		writer.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; form-action "+start.IDPURL)
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = writer.Write(start.PostHTML)
		return
	}
	http.Redirect(writer, request, start.RedirectURL, http.StatusSeeOther)
}

func (login *SAMLHTTP[T]) unlinkAPI(writer http.ResponseWriter, request *http.Request) {
	if err := login.profiles.Unlink(SAMLProtocol, login.hooks.CurrentProfileID(request)); err != nil {
		login.hooks.APIError(writer, err, http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (login *SAMLHTTP[T]) unlinkWeb(writer http.ResponseWriter, request *http.Request) {
	if err := login.profiles.Unlink(SAMLProtocol, login.hooks.CurrentProfileID(request)); err != nil {
		login.hooks.Error(writer, request, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(writer, request, "/account", http.StatusSeeOther)
}
