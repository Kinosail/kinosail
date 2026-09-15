package auth

import (
	"context"

	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/webauthn"
)

// PasskeyOwner is the Owner identity exposed to WebAuthn.
type PasskeyOwner = sharedpasskeys.User[struct{}]

// PasskeyOwner returns a safe copy of the configured Owner.
func (manager *Manager) PasskeyOwner() (PasskeyOwner, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID == "" {
		return PasskeyOwner{}, ErrNotConfigured
	}
	return passkeyOwner(manager.owner), nil
}

// DiscoverPasskey finds the Owner for a discoverable credential.
func (manager *Manager) DiscoverPasskey(rawID, userHandle []byte) (webauthn.User, error) {
	if sharedpasskeys.ValidateLookup(rawID, userHandle) != nil {
		return nil, ErrInvalidCredential
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID != string(userHandle) {
		return nil, ErrInvalidCredential
	}
	if sharedpasskeys.Index(manager.owner.Passkeys, rawID) >= 0 {
		return passkeyOwner(manager.owner), nil
	}
	return nil, ErrInvalidCredential
}

// AddPasskey stores one validated Owner credential.
func (manager *Manager) AddPasskey(ctx context.Context, credential *webauthn.Credential) error {
	if sharedpasskeys.ValidateCredential(credential) != nil {
		return ErrInvalidCredential
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID == "" {
		return ErrNotConfigured
	}
	if len(manager.owner.Passkeys) >= sharedpasskeys.MaxCredentials {
		return ErrState
	}
	if sharedpasskeys.Index(manager.owner.Passkeys, credential.ID) >= 0 {
		return ErrInvalidCredential
	}
	previous := sharedpasskeys.CloneAll(manager.owner.Passkeys)
	manager.owner.Passkeys = append(manager.owner.Passkeys, sharedpasskeys.Clone(*credential))
	if err := manager.store.SaveJSON(context.WithoutCancel(ctx), ownerDocument, manager.owner); err != nil {
		manager.owner.Passkeys = previous
		return ErrState
	}
	return nil
}

// LoginWithPasskey updates the credential counter and creates a browser session.
func (manager *Manager) LoginWithPasskey(ctx context.Context, ownerID string, credential *webauthn.Credential, device string) (Session, error) {
	if sharedpasskeys.ValidateCredential(credential) != nil || ownerID == "" {
		return Session{}, ErrInvalidCredential
	}
	device, err := boundedText("device", device, 80)
	if err != nil {
		return Session{}, ErrInvalidCredential
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID != ownerID {
		return Session{}, ErrInvalidCredential
	}
	credentialIndex := sharedpasskeys.Index(manager.owner.Passkeys, credential.ID)
	if credentialIndex < 0 {
		return Session{}, ErrInvalidCredential
	}
	previousOwner, previousSessions := manager.owner, append([]sessionState(nil), manager.sessions...)
	previousOwner.Passkeys = sharedpasskeys.CloneAll(manager.owner.Passkeys)
	manager.owner.Passkeys[credentialIndex] = sharedpasskeys.Clone(*credential)
	manager.removeExpired(manager.now().UTC())
	session, state := manager.newSession(device, "browser")
	manager.sessions = append([]sessionState{state}, manager.sessions...)
	if len(manager.sessions) > maxSessions {
		manager.sessions = manager.sessions[:maxSessions]
	}
	if err := manager.save(ctx); err != nil {
		manager.owner, manager.sessions = previousOwner, previousSessions
		return Session{}, ErrState
	}
	session.ID, session.Name = manager.owner.ID, manager.owner.Name
	return session, nil
}

func passkeyOwner(owner ownerState) PasskeyOwner {
	return sharedpasskeys.NewUser(struct{}{}, owner.ID, owner.Name, owner.Passkeys)
}

func validPasskeys(credentials []webauthn.Credential) bool {
	return sharedpasskeys.ValidateCredentials(credentials) == nil
}
