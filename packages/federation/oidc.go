// Package federation owns the shared OpenID Connect and SAML protocol flows.
package federation

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

const (
	maxPendingStates = 256
	transactionTTL   = 10 * time.Minute
	// OIDCStateCookieName is the authorization state cookie shared by all app adapters.
	OIDCStateCookieName = "kinosail_oidc_state"
)

var (
	ErrNotConfigured       = errors.New("federation is not configured")
	ErrTooManyPending      = errors.New("too many federation requests are pending")
	ErrInvalidCallback     = errors.New("federation callback is invalid")
	ErrInvalidState        = errors.New("federation state is invalid or expired")
	ErrAuthorizationDenied = errors.New("federation authorization was denied")
	ErrProviderUnavailable = errors.New("federation provider is unavailable")
	ErrCodeExchange        = errors.New("federation code exchange failed")
	ErrTokenInvalid        = errors.New("federation token is invalid")
)

// Identity is a verified provider identity.
type Identity struct{ Issuer, Subject string }

// OIDCConfig configures an OpenID Connect authorization-code flow.
type OIDCConfig struct{ Issuer, ClientID, ClientSecret, RedirectURL, IdentityClaim string }

type oidcTransaction struct {
	nonce, verifier, profileID string
	expires                    time.Time
}

type mfaChallenge struct {
	profileID string
	expires   time.Time
}

// OIDC owns provider discovery, authorization state, token exchange, and identity validation.
type OIDC struct {
	config     OIDCConfig
	mu         sync.Mutex
	providerMu sync.Mutex
	provider   *oidc.Provider
	pending    map[string]oidcTransaction
	mfa        map[string]mfaChallenge
}

// OIDCCallback is the verified result of one authorization response.
type OIDCCallback struct {
	Identity    Identity
	ProfileID   string
	ClearCookie bool
}

// NewOIDC creates an isolated OpenID Connect flow.
func NewOIDC(config OIDCConfig) *OIDC {
	if config.IdentityClaim == "" {
		config.IdentityClaim = "sub"
	}
	return &OIDC{config: config, pending: make(map[string]oidcTransaction), mfa: make(map[string]mfaChallenge)}
}

// Configured reports whether all required values are bounded and trusted.
func (flow *OIDC) Configured() bool {
	if flow == nil || !validOIDCClient(flow.config) {
		return false
	}
	issuer, issuerErr := url.Parse(flow.config.Issuer)
	redirect, redirectErr := url.Parse(flow.config.RedirectURL)
	return issuerErr == nil && redirectErr == nil && TrustedURL(issuer) && TrustedURL(redirect)
}

func validOIDCClient(config OIDCConfig) bool {
	return boundedURLText(config.Issuer) && boundedURLText(config.RedirectURL) && config.ClientID != "" && BoundedIdentityText(config.ClientID, 512) && config.ClientSecret != "" && BoundedIdentityText(config.ClientSecret, 4096) && ValidIdentityField(config.IdentityClaim)
}

// Begin creates one bounded authorization transaction and returns its redirect URL and cookie state.
func (flow *OIDC) Begin(ctx context.Context, profileID string) (string, string, error) {
	provider, err := flow.getProvider(ctx)
	if err != nil {
		return "", "", err
	}
	state, nonce, verifier := rand.Text(), rand.Text(), oauth2.GenerateVerifier()
	now := time.Now()
	flow.mu.Lock()
	prune(flow.pending, now, func(transaction oidcTransaction) time.Time { return transaction.expires })
	if len(flow.pending) >= maxPendingStates {
		flow.mu.Unlock()
		return "", "", ErrTooManyPending
	}
	flow.pending[tokenKey(state)] = oidcTransaction{nonce: nonce, verifier: verifier, profileID: profileID, expires: now.Add(transactionTTL)}
	flow.mu.Unlock()
	config := flow.oauthConfig(provider)
	location := config.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	return location, state, nil
}

// Complete validates one callback before it consumes state or contacts the token endpoint.
func (flow *OIDC) Complete(ctx context.Context, rawQuery, cookieState string, cookiePresent bool) (OIDCCallback, error) {
	code, responseIssuer, providerError, valid := parseOIDCCallbackQuery(rawQuery)
	if !valid || responseIssuer != "" && responseIssuer != flow.config.Issuer {
		return OIDCCallback{}, ErrInvalidCallback
	}
	transaction, validState := flow.takeTransaction(parseState(rawQuery), cookieState, cookiePresent)
	result := OIDCCallback{ProfileID: transaction.profileID, ClearCookie: cookiePresent}
	if !validState {
		return result, ErrInvalidState
	}
	if providerError {
		return result, ErrAuthorizationDenied
	}
	provider, err := flow.getProvider(ctx)
	if err != nil {
		return result, ErrProviderUnavailable
	}
	config := flow.oauthConfig(provider)
	token, err := config.Exchange(ctx, code, oauth2.VerifierOption(transaction.verifier))
	raw, ok := tokenExtra(token, err)
	if !ok {
		return result, ErrCodeExchange
	}
	result.Identity, err = flow.verifiedIdentity(ctx, provider, raw, transaction.nonce)
	if err != nil {
		return result, ErrTokenInvalid
	}
	return result, nil
}

// BeginMFA creates one bounded challenge for a verified federated profile.
func (flow *OIDC) BeginMFA(profileID string) (string, error) {
	challenge, now := rand.Text(), time.Now()
	flow.mu.Lock()
	defer flow.mu.Unlock()
	prune(flow.mfa, now, func(entry mfaChallenge) time.Time { return entry.expires })
	if len(flow.mfa) >= maxPendingStates {
		return "", ErrTooManyPending
	}
	flow.mfa[tokenKey(challenge)] = mfaChallenge{profileID: profileID, expires: now.Add(transactionTTL)}
	return challenge, nil
}

// MFAProfile returns the profile for one active challenge without consuming it.
func (flow *OIDC) MFAProfile(challenge string) (string, bool) {
	flow.mu.Lock()
	defer flow.mu.Unlock()
	entry, found := flow.mfa[tokenKey(challenge)]
	return entry.profileID, found && time.Now().Before(entry.expires)
}

// ConsumeMFA removes one active challenge after successful local verification.
func (flow *OIDC) ConsumeMFA(challenge string) (string, bool) {
	flow.mu.Lock()
	defer flow.mu.Unlock()
	key := tokenKey(challenge)
	entry, found := flow.mfa[key]
	if found {
		delete(flow.mfa, key)
	}
	return entry.profileID, found && time.Now().Before(entry.expires)
}

// StateCookie returns the short-lived, HTTP-only authorization state cookie.
func StateCookie(state string, secure bool) *http.Cookie {
	maxAge := 600
	if state == "" {
		maxAge = -1
	}
	//nolint:gosec // Production TLS supplies Secure; explicit loopback HTTP remains supported.
	return &http.Cookie{Name: OIDCStateCookieName, Value: state, Path: "/login/oidc", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode}
}

func (flow *OIDC) takeTransaction(state, cookieState string, cookiePresent bool) (oidcTransaction, bool) {
	flow.mu.Lock()
	defer flow.mu.Unlock()
	key := tokenKey(state)
	transaction, found := flow.pending[key]
	now := time.Now()
	valid := found && now.Before(transaction.expires) && cookiePresent && sameState(cookieState, state)
	if found && (now.After(transaction.expires) || valid) {
		delete(flow.pending, key)
	}
	return transaction, valid
}

func (flow *OIDC) getProvider(ctx context.Context) (*oidc.Provider, error) {
	flow.providerMu.Lock()
	defer flow.providerMu.Unlock()
	if flow.provider != nil {
		return flow.provider, nil
	}
	if !flow.Configured() {
		return nil, ErrNotConfigured
	}
	provider, err := oidc.NewProvider(ctx, flow.config.Issuer)
	if err != nil {
		return nil, ErrProviderUnavailable
	}
	if err = validateOIDCProvider(provider); err != nil {
		return nil, err
	}
	flow.provider = provider
	return provider, nil
}

func (flow *OIDC) oauthConfig(provider *oidc.Provider) oauth2.Config {
	scopes := []string{oidc.ScopeOpenID}
	if flow.config.IdentityClaim != "sub" {
		scopes = append(scopes, "profile")
	}
	return oauth2.Config{ClientID: flow.config.ClientID, ClientSecret: flow.config.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: flow.config.RedirectURL, Scopes: scopes}
}

func (flow *OIDC) verifiedIdentity(ctx context.Context, provider *oidc.Provider, raw, nonce string) (Identity, error) {
	idToken, err := provider.Verifier(&oidc.Config{ClientID: flow.config.ClientID}).Verify(ctx, raw)
	if err != nil || idToken.Nonce != nonce {
		return Identity{}, ErrTokenInvalid
	}
	claims := make(map[string]json.RawMessage)
	var subject string
	if err := idToken.Claims(&claims); err != nil || json.Unmarshal(claims[flow.config.IdentityClaim], &subject) != nil || !ValidSubject(subject) {
		return Identity{}, ErrTokenInvalid
	}
	return Identity{Issuer: idToken.Issuer, Subject: subject}, nil
}
