package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// ChangePassword replaces the Owner password and keeps only the presented browser session.
func (manager *Manager) ChangePassword(ctx context.Context, currentPassword, newPassword, confirmation, presentedToken string) error {
	if !validCurrentPassword(currentPassword) || !validToken(presentedToken, 64) {
		return ErrInvalidCredential
	}
	if err := validateNewPassword(newPassword); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare([]byte(newPassword), []byte(confirmation)) != 1 {
		return errors.New("new password confirmation does not match")
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.owner.ID == "" {
		return ErrNotConfigured
	}
	if bcrypt.CompareHashAndPassword([]byte(manager.owner.PasswordHash), []byte(currentPassword)) != nil {
		return ErrInvalidCredential
	}
	preserved, found := manager.presentedBrowserSession(presentedToken)
	if !found {
		return ErrInvalidCredential
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 12)
	if err != nil {
		return ErrState
	}
	previousOwner, previousSessions := manager.owner, append([]sessionState(nil), manager.sessions...)
	manager.owner.PasswordHash = string(hash)
	manager.sessions = []sessionState{preserved}
	if err := manager.save(ctx); err != nil {
		manager.owner, manager.sessions = previousOwner, previousSessions
		return ErrState
	}
	return nil
}

// presentedBrowserSession requires the manager lock and keeps the last matching live browser session.
func (manager *Manager) presentedBrowserSession(presentedToken string) (sessionState, bool) {
	presentedHash := hashToken(presentedToken)
	var preserved sessionState
	found := false
	now := manager.now().UTC()
	for _, session := range manager.sessions {
		isPresented := subtle.ConstantTimeCompare([]byte(session.TokenHash), []byte(presentedHash)) == 1
		isBrowser := session.Purpose == "" || session.Purpose == "browser"
		if isPresented && isBrowser && session.ExpiresAt.After(now) {
			preserved, found = session, true
		}
	}
	return preserved, found
}

func validCurrentPassword(password string) bool {
	return utf8.ValidString(password) && len(password) >= 1 && len(password) <= 72
}
