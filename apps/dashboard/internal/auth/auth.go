// Package auth owns first-owner setup and durable sessions.
package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
	"golang.org/x/crypto/bcrypt"
)

const (
	ownerDocument    = "owner.json"
	sessionsDocument = "sessions.json"
	SessionCookie    = "kinosail_dashboard_session"
	sessionLifetime  = 30 * 24 * time.Hour
	maxSessions      = 32
)

var (
	ErrAlreadyConfigured = errors.New("dashboard setup is already complete")
	ErrInvalidCredential = errors.New("invalid credentials")
	ErrNotConfigured     = errors.New("dashboard setup is required")
	ErrState             = errors.New("authentication state is unavailable")
)

type documentStore interface {
	LoadJSON(context.Context, string, any) (bool, error)
	SaveJSON(context.Context, string, any) error
	SaveJSONBatch(context.Context, map[string]any) error
}

type ownerState struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	PasswordHash string                `json:"passwordHash"`
	Passkeys     []webauthn.Credential `json:"passkeys,omitempty"`
}

type sessionState struct {
	TokenHash string    `json:"tokenHash"`
	CSRF      string    `json:"csrf"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Device    string    `json:"device"`
	Purpose   string    `json:"purpose,omitempty"`
}

type sessionsState struct {
	Sessions []sessionState `json:"sessions"`
}

// Identity is the authenticated Owner request context.
type Identity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CSRF      string `json:"csrf,omitempty"`
	ViaBearer bool   `json:"-"`
	Purpose   string `json:"-"`
}

// Session is returned only at setup or login.
type Session struct {
	Identity
	Token     string
	ExpiresAt time.Time
}

// Manager owns one Owner and bounded sessions.
type Manager struct {
	mu       sync.Mutex
	store    documentStore
	owner    ownerState
	sessions []sessionState
	now      func() time.Time
}

// NewManager loads durable authentication state.
func NewManager(ctx context.Context, store documentStore) (*Manager, error) {
	if ctx == nil || store == nil {
		return nil, errors.New("authentication state is required")
	}
	manager := &Manager{store: store, now: time.Now}
	ownerFound, err := store.LoadJSON(ctx, ownerDocument, &manager.owner)
	if err != nil {
		return nil, err
	}
	var persisted sessionsState
	sessionsFound, err := store.LoadJSON(ctx, sessionsDocument, &persisted)
	if err != nil {
		return nil, err
	}
	manager.sessions = persisted.Sessions
	if !ownerFound {
		if sessionsFound && len(manager.sessions) != 0 {
			return nil, errors.New("persisted sessions require an Owner")
		}
		return manager, nil
	}
	if err := manager.validatePersisted(); err != nil {
		return nil, err
	}
	if err := manager.persistExpiredSessions(ctx); err != nil {
		return nil, err
	}
	return manager, nil
}

func (manager *Manager) validatePersisted() error {
	if !validToken(manager.owner.ID, 24) || manager.owner.PasswordHash == "" {
		return errors.New("persisted Owner state is invalid")
	}
	if _, err := boundedText("Owner name", manager.owner.Name, 60); err != nil {
		return errors.New("persisted Owner state is invalid")
	}
	if _, err := bcrypt.Cost([]byte(manager.owner.PasswordHash)); err != nil || len(manager.sessions) > maxSessions {
		return errors.New("persisted authentication state is invalid")
	}
	if !validPasskeys(manager.owner.Passkeys) {
		return errors.New("persisted authentication state is invalid")
	}
	for _, session := range manager.sessions {
		if err := validateSession(session); err != nil {
			return errors.New("persisted authentication state is invalid")
		}
	}
	return nil
}

// Configured reports whether first-owner setup is complete.
func (manager *Manager) Configured() bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	return manager.owner.ID != ""
}

// Setup validates all input before creating the first Owner.
func (manager *Manager) Setup(ctx context.Context, name, password, device string) (Session, error) {
	if manager.Configured() {
		return Session{}, ErrAlreadyConfigured
	}
	name, password, device, err := validateCredentials(name, password, device)
	if err != nil {
		return Session{}, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return Session{}, ErrState
	}
	ownerID := randomToken(12)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID != "" {
		return Session{}, ErrAlreadyConfigured
	}
	previousOwner, previousSessions := manager.owner, manager.sessions
	manager.owner = ownerState{ID: ownerID, Name: name, PasswordHash: string(hash)}
	session, state := manager.newSession(device, "browser")
	manager.sessions = []sessionState{state}
	if err := manager.save(ctx); err != nil {
		manager.owner, manager.sessions = previousOwner, previousSessions
		return Session{}, ErrState
	}
	session.ID, session.Name = ownerID, name
	return session, nil
}

// Login verifies credentials and creates a bounded session.
func (manager *Manager) Login(ctx context.Context, name, password, device string) (Session, error) {
	name = strings.TrimSpace(name)
	device, err := boundedText("device", device, 80)
	if err != nil || len(password) == 0 || len(password) > 72 {
		return Session{}, ErrInvalidCredential
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID == "" {
		return Session{}, ErrNotConfigured
	}
	nameMatch := subtle.ConstantTimeCompare([]byte(strings.ToLower(name)), []byte(strings.ToLower(manager.owner.Name))) == 1
	passwordErr := bcrypt.CompareHashAndPassword([]byte(manager.owner.PasswordHash), []byte(password))
	if !nameMatch || passwordErr != nil {
		return Session{}, ErrInvalidCredential
	}
	previousSessions := append([]sessionState(nil), manager.sessions...)
	manager.removeExpired(manager.now().UTC())
	session, state := manager.newSession(device, "browser")
	manager.sessions = append([]sessionState{state}, manager.sessions...)
	if len(manager.sessions) > maxSessions {
		manager.sessions = manager.sessions[:maxSessions]
	}
	if err := manager.saveSessions(ctx); err != nil {
		manager.sessions = previousSessions
		return Session{}, ErrState
	}
	session.ID, session.Name = manager.owner.ID, manager.owner.Name
	return session, nil
}

// Authenticate validates a cookie or bearer token.
func (manager *Manager) Authenticate(ctx context.Context, request *http.Request) (Identity, bool) {
	if request == nil {
		return Identity{}, false
	}
	token, bearer := bearerToken(request.Header.Get("Authorization"))
	if token == "" {
		cookie, err := request.Cookie(SessionCookie)
		if err != nil {
			return Identity{}, false
		}
		token = cookie.Value
	}
	if !validToken(token, 64) {
		return Identity{}, false
	}
	hash := hashToken(token)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	now, changed := manager.now().UTC(), manager.removeExpired(manager.now().UTC())
	for _, session := range manager.sessions {
		if subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(hash)) == 1 && session.ExpiresAt.After(now) {
			if changed {
				_ = manager.saveSessions(context.WithoutCancel(ctx))
			}
			return Identity{ID: manager.owner.ID, Name: manager.owner.Name, CSRF: session.CSRF, ViaBearer: bearer, Purpose: session.Purpose}, true
		}
	}
	if changed {
		_ = manager.saveSessions(context.WithoutCancel(ctx))
	}
	return Identity{}, false
}

// Logout removes the presented session when it exists.
func (manager *Manager) Logout(ctx context.Context, request *http.Request) error {
	if _, ok := manager.Authenticate(ctx, request); !ok {
		return nil
	}
	token, _ := bearerToken(request.Header.Get("Authorization"))
	if token == "" {
		cookie, _ := request.Cookie(SessionCookie) // Authenticate already validated this request's cookie.
		token = cookie.Value
	}
	hash := hashToken(token)
	manager.mu.Lock()
	defer manager.mu.Unlock()
	previous := append([]sessionState(nil), manager.sessions...)
	kept := manager.sessions[:0]
	for _, session := range manager.sessions {
		if subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(hash)) != 1 {
			kept = append(kept, session)
		}
	}
	manager.sessions = kept
	if err := manager.saveSessions(context.WithoutCancel(ctx)); err != nil {
		manager.sessions = previous
		return err
	}
	return nil
}
