package server

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail-dashboard/internal/auth"
	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/protocol"
)

type passkeyAuth struct {
	engine         *sharedpasskeys.Engine
	authentication *auth.Manager
	origin         string
	secure         bool
	limiter        *loginLimiter
	err            error
}

func newPasskeyAuth(rawURL string, authentication *auth.Manager, secure bool) *passkeyAuth {
	engine, err := sharedpasskeys.New(sharedpasskeys.Config{URL: rawURL, DefaultURL: "http://localhost:38400", DisplayName: "Kinosail Dashboard", CookieName: "kinosail_dashboard_passkey"})
	origin := ""
	if engine != nil {
		origin = engine.Origin().String()
	}
	return &passkeyAuth{engine: engine, authentication: authentication, origin: origin, secure: secure, limiter: newLoginLimiter(), err: err}
}

func (passkeys *passkeyAuth) register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/passkeys/register/begin", passkeys.beginRegistration)
	mux.HandleFunc("POST /api/v1/passkeys/register/finish", passkeys.finishRegistration)
	mux.HandleFunc("POST /api/v1/passkeys/login/begin", passkeys.beginLogin)
	mux.HandleFunc("POST /api/v1/passkeys/login/finish", passkeys.finishLogin)
}

func (passkeys *passkeyAuth) beginRegistration(writer http.ResponseWriter, request *http.Request) {
	_, found := registrationIdentity(writer, request)
	if !found {
		return
	}
	if !emptyPasskeyRequest(writer, request) || !passkeys.available(writer, "registration") || !passkeys.requireOrigin(writer, request, "/") {
		return
	}
	owner, err := passkeys.authentication.PasskeyOwner()
	if err != nil {
		apiError(writer, auth.ErrState, http.StatusServiceUnavailable)
		return
	}
	creation, cookie, err := passkeys.engine.BeginRegistration(owner, passkeys.ceremony(owner.ID))
	if err != nil {
		apiError(writer, errors.New("passkey registration is unavailable"), http.StatusServiceUnavailable)
		return
	}
	http.SetCookie(writer, cookie) //nolint:gosec // Secure follows the validated public URL scheme.
	sharedpasskeys.WriteJSON(writer, creation)
}

func (passkeys *passkeyAuth) finishRegistration(writer http.ResponseWriter, request *http.Request) {
	identity, found := registrationIdentity(writer, request)
	if !found {
		return
	}
	if !passkeys.available(writer, "registration") || !passkeys.requireOrigin(writer, request, "/") {
		return
	}
	var response protocol.CredentialCreationResponse
	if !readJSON(writer, request, &response, 64<<10) {
		return
	}
	parsed, err := response.Parse()
	if err != nil {
		apiError(writer, errors.New("passkey registration failed"), http.StatusBadRequest)
		return
	}
	owner, err := passkeys.authentication.PasskeyOwner()
	if err != nil {
		apiError(writer, auth.ErrState, http.StatusServiceUnavailable)
		return
	}
	credential, err := passkeys.engine.FinishRegistration(owner, identity.ID, request, parsed)
	if errors.Is(err, sharedpasskeys.ErrCeremonyExpired) {
		apiError(writer, errors.New("passkey ceremony expired"), http.StatusBadRequest)
		return
	}
	if err != nil || passkeys.authentication.AddPasskey(request.Context(), credential) != nil {
		apiError(writer, errors.New("passkey registration failed"), http.StatusBadRequest)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func (passkeys *passkeyAuth) beginLogin(writer http.ResponseWriter, request *http.Request) {
	if !emptyPasskeyRequest(writer, request) || !passkeys.available(writer, "login") || !passkeys.requireOrigin(writer, request, "/login") {
		return
	}
	allowed, retryAfter := passkeys.limiter.allow(request, "passkey")
	if !allowed {
		writer.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		apiError(writer, errors.New("too many login attempts"), http.StatusTooManyRequests)
		return
	}
	assertion, cookie, err := passkeys.engine.BeginLogin(passkeys.ceremony(""))
	if err != nil {
		apiError(writer, errors.New("passkey login is unavailable"), http.StatusServiceUnavailable)
		return
	}
	passkeys.limiter.success(request, "passkey")
	http.SetCookie(writer, cookie) //nolint:gosec // Secure follows the validated public URL scheme.
	sharedpasskeys.WriteJSON(writer, assertion)
}

func (passkeys *passkeyAuth) finishLogin(writer http.ResponseWriter, request *http.Request) {
	if !passkeys.available(writer, "login") || !passkeys.requireOrigin(writer, request, "/login") {
		return
	}
	var response protocol.CredentialAssertionResponse
	if !readJSON(writer, request, &response, 64<<10) {
		return
	}
	parsed, err := response.Parse()
	if err != nil {
		apiError(writer, auth.ErrInvalidCredential, http.StatusUnauthorized)
		return
	}
	user, credential, err := passkeys.engine.FinishLogin(request, parsed, passkeys.authentication.DiscoverPasskey)
	if errors.Is(err, sharedpasskeys.ErrCeremonyExpired) {
		apiError(writer, errors.New("passkey ceremony expired"), http.StatusBadRequest)
		return
	}
	owner, valid := user.(auth.PasskeyOwner)
	if err != nil || !valid {
		apiError(writer, auth.ErrInvalidCredential, http.StatusUnauthorized)
		return
	}
	login, err := passkeys.authentication.LoginWithPasskey(request.Context(), owner.ID, credential, passkeyDevice(request))
	if err != nil {
		apiError(writer, auth.ErrInvalidCredential, http.StatusUnauthorized)
		return
	}
	passkeys.limiter.success(request, "passkey")
	setSessionCookie(writer, login, passkeys.secure)
	writer.Header().Set("X-Kinosail-Login-Next", "/")
	writer.WriteHeader(http.StatusNoContent)
}

func (passkeys *passkeyAuth) available(writer http.ResponseWriter, kind string) bool {
	if passkeys.err == nil && passkeys.engine != nil {
		return true
	}
	apiError(writer, errors.New("passkey "+kind+" is unavailable"), http.StatusServiceUnavailable)
	return false
}

func (passkeys *passkeyAuth) ceremony(ownerID string) sharedpasskeys.Ceremony {
	return sharedpasskeys.Ceremony{Identity: ownerID, CookiePath: "/api/v1/passkeys/", Secure: passkeys.secure}
}

func (passkeys *passkeyAuth) requireOrigin(writer http.ResponseWriter, request *http.Request, page string) bool {
	if passkeys.engine != nil && passkeys.engine.Origin().Matches(request, passkeys.secure) {
		return true
	}
	writer.Header().Set("Location", passkeys.origin+page)
	apiError(writer, errors.New("use the configured Dashboard address for passkeys"), http.StatusMisdirectedRequest)
	return false
}

func registrationIdentity(writer http.ResponseWriter, request *http.Request) (auth.Identity, bool) {
	identity, found := requireOwner(writer, request)
	if !found {
		return auth.Identity{}, false
	}
	if identity.ViaBearer {
		apiError(writer, errors.New("passkey registration requires a browser session"), http.StatusForbidden)
		return auth.Identity{}, false
	}
	return identity, requireCSRF(writer, request, identity)
}

func passkeyDevice(request *http.Request) string {
	device := strings.TrimSpace(request.UserAgent())
	if device == "" {
		return "Browser"
	}
	if len(device) > 80 {
		return device[:80]
	}
	return device
}

func emptyPasskeyRequest(writer http.ResponseWriter, request *http.Request) bool {
	data, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, 1))
	if err != nil || len(data) != 0 {
		apiError(writer, errors.New("request body must be empty"), http.StatusBadRequest)
		return false
	}
	return true
}
