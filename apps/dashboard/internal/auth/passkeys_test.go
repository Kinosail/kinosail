package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func testCredential(id string) *webauthn.Credential {
	return &webauthn.Credential{ID: []byte(id), PublicKey: []byte("bounded-public-key")}
}

func setupPasskeyManager(t *testing.T) (*Manager, *fakeDocumentStore) {
	t.Helper()
	store := newFakeDocumentStore(t, nil)
	manager, err := NewManager(t.Context(), store)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Setup(t.Context(), "Owner", "long-password-123", "test browser"); err != nil {
		t.Fatal(err)
	}
	return manager, store
}

func passkeyCount(t *testing.T, manager *Manager) int {
	t.Helper()
	owner, err := manager.PasskeyOwner()
	if err != nil {
		t.Fatal(err)
	}
	return len(owner.Credentials)
}

func TestPasskeyRegistrationAndLoginPersistOneBoundedCredential(t *testing.T) {
	manager, store := setupPasskeyManager(t)
	credential := testCredential("credential-one")
	if err := manager.AddPasskey(t.Context(), credential); err != nil {
		t.Fatal(err)
	}
	owner, err := manager.PasskeyOwner()
	if err != nil || len(owner.Credentials) != 1 {
		t.Fatalf("Owner credentials = %d, %v", len(owner.Credentials), err)
	}
	credential.ID[0] = 'X'
	if owner.Credentials[0].ID[0] == 'X' {
		t.Fatal("stored credential aliases caller memory")
	}
	user, err := manager.DiscoverPasskey(owner.Credentials[0].ID, []byte(owner.ID))
	if err != nil || user.WebAuthnName() != "Owner" {
		t.Fatalf("discover = %#v, %v", user, err)
	}
	if _, err := manager.LoginWithPasskey(t.Context(), owner.ID, &owner.Credentials[0], "passkey browser"); err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewManager(t.Context(), store)
	if err != nil || passkeyCount(t, reloaded) != 1 {
		t.Fatalf("reloaded passkeys = %d, %v", passkeyCount(t, reloaded), err)
	}
}

func TestRejectedPasskeysCauseNoSideEffects(t *testing.T) {
	manager, store := setupPasskeyManager(t)
	for _, credential := range []*webauthn.Credential{nil, {}, {ID: []byte("id")}, {ID: make([]byte, 1025), PublicKey: []byte("key")}, {ID: []byte("id"), PublicKey: make([]byte, 8193)}, {ID: []byte("id"), PublicKey: []byte("key"), Transport: []protocol.AuthenticatorTransport{"unknown"}}, {ID: []byte("id"), PublicKey: []byte("key"), Attestation: webauthn.CredentialAttestation{Object: make([]byte, (64<<10)+1)}}} {
		if err := manager.AddPasskey(t.Context(), credential); !errors.Is(err, ErrInvalidCredential) {
			t.Fatalf("invalid credential error = %v", err)
		}
		if passkeyCount(t, manager) != 0 {
			t.Fatal("invalid credential changed passkeys")
		}
	}
	credential := testCredential("credential-one")
	if err := manager.AddPasskey(t.Context(), credential); err != nil {
		t.Fatal(err)
	}
	if err := manager.AddPasskey(t.Context(), credential); !errors.Is(err, ErrInvalidCredential) || passkeyCount(t, manager) != 1 {
		t.Fatalf("duplicate result = %v, count = %d", err, passkeyCount(t, manager))
	}
	store.failSave = true
	if err := manager.AddPasskey(context.Background(), testCredential("credential-two")); !errors.Is(err, ErrState) || passkeyCount(t, manager) != 1 {
		t.Fatalf("failed save result = %v, count = %d", err, passkeyCount(t, manager))
	}
}

func TestPasskeyLoginRollsBackCredentialAndSessionsWhenSaveFails(t *testing.T) {
	manager, store := setupPasskeyManager(t)
	credential := testCredential("credential-one")
	if err := manager.AddPasskey(t.Context(), credential); err != nil {
		t.Fatal(err)
	}
	owner, _ := manager.PasskeyOwner()
	store.failBatch = true
	if _, err := manager.LoginWithPasskey(t.Context(), owner.ID, &owner.Credentials[0], "passkey browser"); !errors.Is(err, ErrState) {
		t.Fatalf("login error = %v", err)
	}
	store.failBatch = false
	if _, err := manager.LoginWithPasskey(t.Context(), owner.ID, &owner.Credentials[0], "passkey browser"); err != nil {
		t.Fatalf("login after rollback: %v", err)
	}
}
