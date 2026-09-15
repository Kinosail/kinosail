package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type fakeDocumentStore struct {
	documents map[string][]byte
	failSave  bool
	failBatch bool
}

func newFakeDocumentStore(t *testing.T, documents map[string]any) *fakeDocumentStore {
	t.Helper()
	store := &fakeDocumentStore{documents: make(map[string][]byte, len(documents))}
	for name, value := range documents {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		store.documents[name] = data
	}
	return store
}

func (store *fakeDocumentStore) LoadJSON(_ context.Context, name string, target any) (bool, error) {
	data, ok := store.documents[name]
	if !ok {
		return false, nil
	}
	return true, json.Unmarshal(data, target)
}

func (store *fakeDocumentStore) SaveJSON(_ context.Context, name string, value any) error {
	if store.failSave {
		return errors.New("injected SaveJSON failure")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	store.documents[name] = data
	return nil
}

func (store *fakeDocumentStore) SaveJSONBatch(_ context.Context, values map[string]any) error {
	if store.failBatch {
		return errors.New("injected SaveJSONBatch failure")
	}
	encoded := make(map[string][]byte, len(values))
	for name, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		encoded[name] = data
	}
	for name, data := range encoded {
		store.documents[name] = data
	}
	return nil
}

func TestSetupRollsBackOwnerAndSessionWhenBatchSaveFails(t *testing.T) {
	ctx := context.Background()
	store := newFakeDocumentStore(t, nil)
	manager, err := NewManager(ctx, store)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	store.failBatch = true

	_, err = manager.Setup(ctx, "Owner", "correct horse battery", "test browser")
	if !errors.Is(err, ErrState) {
		t.Fatalf("Setup error = %v, want ErrState", err)
	}
	if manager.Configured() {
		t.Fatal("manager remained configured after failed atomic setup")
	}
	if len(manager.sessions) != 0 {
		t.Fatalf("sessions after failed setup = %d, want 0", len(manager.sessions))
	}
	if len(store.documents) != 0 {
		t.Fatalf("persisted documents after failed setup = %v, want none", store.documents)
	}
}

func TestSetupRejectsPasswordsOverBcryptLimitWithoutSaving(t *testing.T) {
	for name, password := range map[string]string{
		"ASCII":   strings.Repeat("a", 73),
		"Unicode": strings.Repeat("界", 25),
	} {
		t.Run(name, func(t *testing.T) {
			store := newFakeDocumentStore(t, nil)
			manager, err := NewManager(context.Background(), store)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := manager.Setup(context.Background(), "Owner", password, "browser"); err == nil || errors.Is(err, ErrState) {
				t.Fatalf("setup error = %v, want validation error", err)
			}
			if manager.Configured() || len(store.documents) != 0 {
				t.Fatal("rejected password changed authentication state")
			}
		})
	}
}

func TestLoginAtSessionLimitRestoresAllSessionsWhenSaveFails(t *testing.T) {
	ctx := context.Background()
	password := "correct horse battery"
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	sessions := make([]sessionState, maxSessions)
	for index := range sessions {
		token := repeatedHex(index+1, 64)
		sessions[index] = sessionState{
			TokenHash: token,
			CSRF:      repeatedHex(index+33, 48),
			CreatedAt: now.Add(-time.Duration(index+1) * time.Hour),
			ExpiresAt: now.Add(24 * time.Hour),
			Device:    "existing browser",
		}
	}
	store := newFakeDocumentStore(t, nil)
	manager := &Manager{
		store:    store,
		owner:    ownerState{ID: repeatedHex(7, 24), Name: "Owner", PasswordHash: string(passwordHash)},
		sessions: append([]sessionState(nil), sessions...),
		now:      func() time.Time { return now },
	}
	store.failSave = true

	_, err = manager.Login(ctx, "owner", password, "new browser")
	if !errors.Is(err, ErrState) {
		t.Fatalf("Login error = %v, want ErrState", err)
	}
	if !reflect.DeepEqual(manager.sessions, sessions) {
		t.Fatal("failed login did not restore the complete capped session list")
	}
}

func TestLogoutRestoresSessionWhenSaveFails(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	token := repeatedHex(11, 64)
	session := sessionState{
		TokenHash: hashToken(token),
		CSRF:      repeatedHex(13, 48),
		CreatedAt: now.Add(-time.Hour),
		ExpiresAt: now.Add(time.Hour),
		Device:    "test browser",
	}
	store := newFakeDocumentStore(t, nil)
	manager := &Manager{
		store:    store,
		owner:    ownerState{ID: repeatedHex(17, 24), Name: "Owner", PasswordHash: "unused"},
		sessions: []sessionState{session},
		now:      func() time.Time { return now },
	}
	store.failSave = true
	request := httptest.NewRequestWithContext(t.Context(), "POST", "/api/v1/session/logout", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookie, Value: token}) // #nosec G124 -- Incoming request cookie; AddCookie sends only its name and value.

	err := manager.Logout(ctx, request)
	if err == nil {
		t.Fatal("Logout error = nil, want injected persistence error")
	}
	if !reflect.DeepEqual(manager.sessions, []sessionState{session}) {
		t.Fatal("failed logout did not restore the authenticated session")
	}
}

func TestNewManagerRejectsInvalidPersistedState(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	validOwner := ownerState{ID: repeatedHex(19, 24), Name: "Owner", PasswordHash: validPasswordHash(t)}
	validSession := sessionState{
		TokenHash: repeatedHex(21, 64),
		CSRF:      repeatedHex(22, 48),
		CreatedAt: now,
		ExpiresAt: now.Add(time.Hour),
		Device:    "test browser",
	}
	tests := []struct {
		name      string
		documents map[string]any
	}{
		{
			name: "sessions without owner",
			documents: map[string]any{
				sessionsDocument: sessionsState{Sessions: []sessionState{validSession}},
			},
		},
		{
			name: "invalid owner identifier",
			documents: map[string]any{
				ownerDocument: ownerState{ID: "not-a-token", Name: validOwner.Name, PasswordHash: validOwner.PasswordHash},
			},
		},
		{
			name: "too many sessions",
			documents: map[string]any{
				ownerDocument:    validOwner,
				sessionsDocument: sessionsState{Sessions: repeatSession(validSession, maxSessions+1)},
			},
		},
		{
			name: "invalid session lifetime",
			documents: map[string]any{
				ownerDocument: validOwner,
				sessionsDocument: sessionsState{Sessions: []sessionState{{
					TokenHash: validSession.TokenHash,
					CSRF:      validSession.CSRF,
					CreatedAt: now,
					ExpiresAt: now.Add(sessionLifetime + time.Second),
					Device:    validSession.Device,
				}}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewManager(context.Background(), newFakeDocumentStore(t, test.documents))
			if err == nil {
				t.Fatal("NewManager error = nil, want invalid-state error")
			}
		})
	}
}

func validPasswordHash(t *testing.T) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return string(hash)
}

func repeatSession(session sessionState, count int) []sessionState {
	sessions := make([]sessionState, count)
	for index := range sessions {
		sessions[index] = session
	}
	return sessions
}

func repeatedHex(seed, length int) string {
	digits := "0123456789abcdef"
	result := make([]byte, length)
	for index := range result {
		result[index] = digits[(seed+index)%len(digits)]
	}
	return string(result)
}
