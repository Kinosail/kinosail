package auth

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	sharedpasskeys "github.com/MikeO7/kinosail/packages/passkeys"
	"github.com/go-webauthn/webauthn/webauthn"
)

func TestCoverageManagerLoadFailures(t *testing.T) {
	if _, err := NewManager(nil, newFakeDocumentStore(t, nil)); err == nil { //nolint:staticcheck // Verify the public nil-context rejection contract.
		t.Fatal("nil context accepted")
	}
	if _, err := NewManager(t.Context(), nil); err == nil {
		t.Fatal("nil storage accepted")
	}
	for _, document := range []string{ownerDocument, sessionsDocument} {
		store := newFakeDocumentStore(t, nil)
		store.documents[document] = []byte("{")
		if _, err := NewManager(t.Context(), store); err == nil {
			t.Fatalf("corrupt %s accepted", document)
		}
	}
	owner := ownerState{ID: strings.Repeat("1", 24), Name: "", PasswordHash: validPasswordHash(t)}
	if _, err := NewManager(t.Context(), newFakeDocumentStore(t, map[string]any{ownerDocument: owner})); err == nil {
		t.Fatal("empty owner name accepted")
	}
	owner.Name = "Owner"
	owner.Passkeys = []webauthn.Credential{{}}
	if _, err := NewManager(t.Context(), newFakeDocumentStore(t, map[string]any{ownerDocument: owner})); err == nil {
		t.Fatal("invalid persisted passkey accepted")
	}
}

func TestCoverageManagerExpiredSessionPersistenceFailure(t *testing.T) {
	now := time.Now()
	owner := ownerState{ID: strings.Repeat("1", 24), Name: "Owner", PasswordHash: validPasswordHash(t)}
	session := sessionState{TokenHash: strings.Repeat("a", 64), CSRF: strings.Repeat("b", 48), CreatedAt: now.Add(-2 * time.Hour), ExpiresAt: now.Add(-time.Hour), Device: "Browser"}
	store := newFakeDocumentStore(t, map[string]any{ownerDocument: owner, sessionsDocument: sessionsState{Sessions: []sessionState{session}}})
	store.failSave = true
	if _, err := NewManager(t.Context(), store); err == nil {
		t.Fatal("expired-session save failure ignored")
	}
}

func TestCoveragePasskeyLookupBoundaries(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	manager.owner.ID = ""
	if _, err := manager.PasskeyOwner(); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing owner = %v", err)
	}
	if err := manager.AddPasskey(t.Context(), testCredential("one")); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("registration without owner = %v", err)
	}
	manager.owner.ID = strings.Repeat("1", 24)
	for _, lookup := range []struct{ id, owner []byte }{
		{}, {[]byte("one"), []byte("wrong")}, {[]byte("unknown"), []byte(manager.owner.ID)},
	} {
		if _, err := manager.DiscoverPasskey(lookup.id, lookup.owner); !errors.Is(err, ErrInvalidCredential) {
			t.Fatalf("lookup = %v", err)
		}
	}
	manager.owner.Passkeys = make([]webauthn.Credential, sharedpasskeys.MaxCredentials)
	if err := manager.AddPasskey(t.Context(), testCredential("one")); !errors.Is(err, ErrState) {
		t.Fatalf("credential capacity = %v", err)
	}
}

func TestCoveragePasskeyLoginBoundaries(t *testing.T) {
	for _, mode := range []string{"credential", "device", "owner", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			manager, store := coverageSessionManager(t)
			credential, owner, device := testCredential("one"), manager.owner.ID, "Browser"
			switch mode {
			case "credential":
				credential = nil
			case "device":
				device = ""
			case "owner":
				owner = "wrong"
			}
			if _, err := manager.LoginWithPasskey(t.Context(), owner, credential, device); !errors.Is(err, ErrInvalidCredential) {
				t.Fatalf("login = %v", err)
			}
			if len(store.documents) != 0 || len(manager.sessions) != 0 {
				t.Fatal("rejected login changed state")
			}
		})
	}
}

func TestCoveragePasskeyLoginCapsSessions(t *testing.T) {
	manager, _ := coverageSessionManager(t)
	credential := testCredential("one")
	manager.owner.Passkeys = []webauthn.Credential{*credential}
	_, state := manager.newSession("Browser", "browser")
	manager.sessions = repeatSession(state, maxSessions)
	session, err := manager.LoginWithPasskey(t.Context(), manager.owner.ID, credential, "Browser")
	if err != nil || session.Token == "" || len(manager.sessions) != maxSessions {
		t.Fatalf("capped login = %+v, %v", session, err)
	}
	if reflect.DeepEqual(manager.sessions[0], state) {
		t.Fatal("new session was not placed first")
	}
}
