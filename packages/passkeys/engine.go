package passkeys

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

const (
	maxCeremonies = 1024
	maxCookieName = 64
	maxCookiePath = 256
)

var (
	ErrInvalidConfig   = errors.New("invalid passkey configuration")
	ErrInvalidCeremony = errors.New("invalid passkey ceremony")
	ErrCeremonyExpired = errors.New("passkey ceremony expired")
	ErrCeremonyLimit   = errors.New("too many passkey ceremonies")
)

// Config defines one WebAuthn engine.
type Config struct {
	URL         string
	DefaultURL  string
	DisplayName string
	CookieName  string
	newWebAuthn func(*webauthn.Config) (*webauthn.WebAuthn, error)
}

// Ceremony defines request-specific cookie and revocation data.
type Ceremony struct {
	Identity   string
	CookiePath string
	Secure     bool
	Public     bool
}

type ceremonyState struct {
	identity string
	session  webauthn.SessionData
	public   bool
}

// Engine owns WebAuthn protocol options and one-use ceremony state.
type Engine struct {
	mu                sync.Mutex
	web               *webauthn.WebAuthn
	ceremonies        map[string]ceremonyState
	origin            Origin
	cookieName        string
	now               func() time.Time
	beginRegistration func(webauthn.User, ...webauthn.RegistrationOption) (*protocol.CredentialCreation, *webauthn.SessionData, error)
	beginLogin        func(...webauthn.LoginOption) (*protocol.CredentialAssertion, *webauthn.SessionData, error)
}

// MatchesRequest reports whether a request uses this engine's configured origin.
func (engine *Engine) MatchesRequest(request *http.Request, secure bool) bool {
	return engine != nil && engine.Origin().Matches(request, secure)
}

// RequirePageOrigin redirects an enabled browser page to this engine's canonical origin.
func (engine *Engine) RequirePageOrigin(writer http.ResponseWriter, request *http.Request, enabled, secure bool) bool {
	if !enabled || engine == nil || engine.MatchesRequest(request, secure) {
		return true
	}
	target, status := engine.Origin().PageRedirect(request)
	http.Redirect(writer, request, target, status)
	return false
}

// New creates an engine from one validated origin and cookie name.
func New(config Config) (*Engine, error) {
	rawURL := config.URL
	if rawURL == "" {
		rawURL = config.DefaultURL
	}
	if config.DisplayName == "" || len(config.DisplayName) > 100 || !validCookieName(config.CookieName) {
		return nil, ErrInvalidConfig
	}
	origin, err := ParseOrigin(rawURL)
	if err != nil {
		return nil, err
	}
	create := config.newWebAuthn
	if create == nil {
		create = webauthn.New
	}
	web, err := create(&webauthn.Config{
		RPID: origin.RPID(), RPDisplayName: config.DisplayName, RPOrigins: []string{origin.String()},
		AuthenticatorSelection: protocol.AuthenticatorSelection{ResidentKey: protocol.ResidentKeyRequirementRequired, RequireResidentKey: protocol.ResidentKeyRequired(), UserVerification: protocol.VerificationRequired},
		Timeouts:               webauthn.TimeoutsConfig{Login: webauthn.TimeoutConfig{Enforce: true}, Registration: webauthn.TimeoutConfig{Enforce: true}},
	})
	if err != nil {
		return nil, err
	}
	return &Engine{web: web, ceremonies: make(map[string]ceremonyState), origin: origin, cookieName: config.CookieName, now: time.Now, beginRegistration: web.BeginRegistration, beginLogin: web.BeginDiscoverableLogin}, nil
}

// Origin returns the engine's validated origin.
func (engine *Engine) Origin() Origin {
	if engine == nil {
		return Origin{}
	}
	return engine.origin
}

// BeginRegistration creates registration data and a server-side ceremony cookie.
func (engine *Engine) BeginRegistration(user webauthn.User, ceremony Ceremony) (*protocol.CredentialCreation, *http.Cookie, error) {
	if engine == nil || user == nil || ceremony.Identity == "" {
		return nil, nil, ErrInvalidCeremony
	}
	creation, session, err := engine.beginRegistration(user, webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired))
	if err != nil {
		return nil, nil, err
	}
	cookie, err := engine.start(ceremony, session)
	return creation, cookie, err
}

// FinishRegistrationRequest verifies a registration response from net/http.
func (engine *Engine) FinishRegistrationRequest(user webauthn.User, identity string, request *http.Request) (*webauthn.Credential, error) {
	session, err := engine.take(request, identity)
	if err != nil {
		return nil, err
	}
	return engine.web.FinishRegistration(user, session, request)
}

// FinishRegistration verifies a strictly parsed registration response.
func (engine *Engine) FinishRegistration(user webauthn.User, identity string, request *http.Request, response *protocol.ParsedCredentialCreationData) (*webauthn.Credential, error) {
	session, err := engine.take(request, identity)
	if err != nil {
		return nil, err
	}
	return engine.web.CreateCredential(user, session, response)
}

// BeginLogin creates discoverable-login data and a server-side ceremony cookie.
func (engine *Engine) BeginLogin(ceremony Ceremony) (*protocol.CredentialAssertion, *http.Cookie, error) {
	if engine == nil || ceremony.Identity != "" {
		return nil, nil, ErrInvalidCeremony
	}
	assertion, session, err := engine.beginLogin()
	if err != nil {
		return nil, nil, err
	}
	cookie, err := engine.start(ceremony, session)
	return assertion, cookie, err
}

// FinishLoginRequest verifies a discoverable login response from net/http.
func (engine *Engine) FinishLoginRequest(request *http.Request, discover webauthn.DiscoverableUserHandler) (webauthn.User, *webauthn.Credential, error) {
	session, err := engine.take(request, "")
	if err != nil {
		return nil, nil, err
	}
	return engine.web.FinishPasskeyLogin(discover, session, request)
}

// FinishLogin verifies a strictly parsed discoverable login response.
func (engine *Engine) FinishLogin(request *http.Request, response *protocol.ParsedCredentialAssertionData, discover webauthn.DiscoverableUserHandler) (webauthn.User, *webauthn.Credential, error) {
	session, err := engine.take(request, "")
	if err != nil {
		return nil, nil, err
	}
	return engine.web.ValidatePasskeyLogin(discover, session, response)
}

// RevokePublic removes every in-flight public ceremony.
func (engine *Engine) RevokePublic() {
	if engine == nil {
		return
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	for id, ceremony := range engine.ceremonies {
		if ceremony.public {
			delete(engine.ceremonies, id)
		}
	}
}

// WriteJSON writes a no-store WebAuthn response.
func WriteJSON(writer http.ResponseWriter, value any) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode(value)
}

func (engine *Engine) start(ceremony Ceremony, session *webauthn.SessionData) (*http.Cookie, error) {
	if session == nil || len(ceremony.Identity) > maxUserHandle || !validCookiePath(ceremony.CookiePath) {
		return nil, ErrInvalidCeremony
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	now := engine.now()
	for id, active := range engine.ceremonies {
		if !active.session.Expires.IsZero() && active.session.Expires.Before(now) {
			delete(engine.ceremonies, id)
		}
	}
	if len(engine.ceremonies) >= maxCeremonies {
		return nil, ErrCeremonyLimit
	}
	id := rand.Text()
	engine.ceremonies[id] = ceremonyState{identity: ceremony.Identity, session: *session, public: ceremony.Public}
	return &http.Cookie{Name: engine.cookieName, Value: id, Path: ceremony.CookiePath, MaxAge: 300, HttpOnly: true, Secure: ceremony.Secure, SameSite: http.SameSiteStrictMode}, nil //nolint:gosec // Plaintext localhost must complete a passkey ceremony.
}

func (engine *Engine) take(request *http.Request, identity string) (webauthn.SessionData, error) {
	if engine == nil || request == nil || len(identity) > maxUserHandle {
		return webauthn.SessionData{}, ErrInvalidCeremony
	}
	cookie, err := ceremonyCookie(request, engine.cookieName)
	if err != nil {
		return webauthn.SessionData{}, ErrCeremonyExpired
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	ceremony, found := engine.ceremonies[cookie.Value]
	delete(engine.ceremonies, cookie.Value)
	if !found || !ceremony.valid(identity, engine.now()) {
		return webauthn.SessionData{}, ErrCeremonyExpired
	}
	return ceremony.session, nil
}

func ceremonyCookie(request *http.Request, name string) (*http.Cookie, error) {
	cookie, err := request.Cookie(name)
	if err != nil || len(cookie.Value) == 0 || len(cookie.Value) > maxCookieName {
		return nil, ErrCeremonyExpired
	}
	return cookie, nil
}

func (ceremony ceremonyState) valid(identity string, now time.Time) bool {
	return ceremony.identity == identity && (ceremony.session.Expires.IsZero() || ceremony.session.Expires.After(now))
}

func validCookieName(name string) bool {
	if name == "" || len(name) > maxCookieName {
		return false
	}
	return !strings.ContainsAny(name, "()<>@,;:\\\"/[]?={} \t\r\n")
}

func validCookiePath(path string) bool {
	return path != "" && len(path) <= maxCookiePath && strings.HasPrefix(path, "/") && !strings.HasPrefix(path, "//") && !strings.ContainsAny(path, ";\r\n")
}
