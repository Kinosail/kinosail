package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

func (manager *Manager) newSession(device, purpose string) (Session, sessionState) {
	token, csrf := randomToken(32), randomToken(24)
	now := manager.now().UTC()
	state := sessionState{TokenHash: hashToken(token), CSRF: csrf, CreatedAt: now, ExpiresAt: now.Add(sessionLifetime), Device: device, Purpose: purpose}
	return Session{Identity: Identity{CSRF: csrf}, Token: token, ExpiresAt: state.ExpiresAt}, state
}

func (manager *Manager) removeExpired(now time.Time) bool {
	kept := manager.sessions[:0]
	for _, session := range manager.sessions {
		if session.ExpiresAt.After(now) && validToken(session.TokenHash, 64) && validToken(session.CSRF, 48) {
			kept = append(kept, session)
		}
	}
	changed := len(kept) != len(manager.sessions)
	manager.sessions = kept
	return changed
}

func (manager *Manager) persistExpiredSessions(ctx context.Context) error {
	if manager.removeExpired(manager.now().UTC()) {
		return manager.saveSessions(ctx)
	}
	return nil
}

func validateSession(session sessionState) error {
	if !validToken(session.TokenHash, 64) || !validToken(session.CSRF, 48) || session.CreatedAt.IsZero() || !session.ExpiresAt.After(session.CreatedAt) || session.ExpiresAt.Sub(session.CreatedAt) > sessionLifetime {
		return errors.New("session is invalid")
	}
	if _, err := boundedText("device", session.Device, 80); err != nil {
		return err
	}
	if session.Purpose != "" && session.Purpose != "browser" && session.Purpose != "mcp" {
		return errors.New("session purpose is invalid")
	}
	return nil
}

func (manager *Manager) save(ctx context.Context) error {
	return manager.store.SaveJSONBatch(context.WithoutCancel(ctx), map[string]any{
		ownerDocument:    manager.owner,
		sessionsDocument: sessionsState{Sessions: manager.sessions},
	})
}

func (manager *Manager) saveSessions(ctx context.Context) error {
	return manager.store.SaveJSON(context.WithoutCancel(ctx), sessionsDocument, sessionsState{Sessions: manager.sessions})
}

func validateCredentials(name, password, device string) (string, string, string, error) {
	name, err := boundedText("name", name, 60)
	if err != nil {
		return "", "", "", err
	}
	device, err = boundedText("device", device, 80)
	if err != nil {
		return "", "", "", err
	}
	if err := validateNewPassword(password); err != nil {
		return "", "", "", err
	}
	return name, password, device, nil
}

func validateNewPassword(password string) error {
	if !utf8.ValidString(password) || len(password) < 12 || len(password) > 72 || strings.TrimSpace(password) != password {
		return errors.New("password must contain 12 to 72 UTF-8 bytes without outer spaces")
	}
	return nil
}

func boundedText(field, value string, maximum int) (string, error) {
	value = strings.TrimSpace(value)
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) < 1 || utf8.RuneCountInString(value) > maximum {
		return "", errors.New(field + " has an invalid length")
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return "", errors.New(field + " contains control characters")
		}
	}
	return value, nil
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1], true
	}
	return "", false
}

func randomToken(size int) string {
	data := make([]byte, size)
	_, _ = rand.Read(data) // Go guarantees success or terminates the process.
	return hex.EncodeToString(data)
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func validToken(token string, length int) bool {
	return len(token) == length && strings.IndexFunc(token, func(character rune) bool {
		return character < '0' || character > '9' && character < 'a' || character > 'f'
	}) == -1
}
