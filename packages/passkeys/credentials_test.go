package passkeys

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

func credential(id string) webauthn.Credential {
	return webauthn.Credential{
		ID:            []byte(id),
		PublicKey:     []byte("public-key"),
		Transport:     []protocol.AuthenticatorTransport{protocol.USB, protocol.NFC, protocol.BLE, protocol.SmartCard, protocol.Hybrid, protocol.Internal},
		Authenticator: webauthn.Authenticator{AAGUID: bytes.Repeat([]byte{1}, 16)},
		Attestation: webauthn.CredentialAttestation{
			ClientDataJSON:    []byte("client"),
			ClientDataHash:    []byte("hash"),
			AuthenticatorData: []byte("authenticator"),
			Object:            []byte("object"),
		},
	}
}

func TestCredentialValidationRejectsEveryBoundWithoutSideEffects(t *testing.T) {
	valid := credential("credential")
	invalid := []*webauthn.Credential{
		nil,
		{},
		{ID: []byte("id")},
		{ID: bytes.Repeat([]byte("i"), maxCredentialID+1), PublicKey: []byte("key")},
		{ID: []byte("id"), PublicKey: bytes.Repeat([]byte("k"), maxPublicKey+1)},
		{ID: []byte("id"), PublicKey: []byte("key"), Transport: make([]protocol.AuthenticatorTransport, maxTransports+1)},
		{ID: []byte("id"), PublicKey: []byte("key"), Transport: []protocol.AuthenticatorTransport{"unknown"}},
		{ID: []byte("id"), PublicKey: []byte("key"), Authenticator: webauthn.Authenticator{AAGUID: make([]byte, 17)}},
		{ID: []byte("id"), PublicKey: []byte("key"), Attestation: webauthn.CredentialAttestation{ClientDataJSON: make([]byte, maxAttestation+1)}},
		{ID: []byte("id"), PublicKey: []byte("key"), Attestation: webauthn.CredentialAttestation{ClientDataHash: make([]byte, 65)}},
		{ID: []byte("id"), PublicKey: []byte("key"), Attestation: webauthn.CredentialAttestation{AuthenticatorData: make([]byte, maxAttestation+1)}},
		{ID: []byte("id"), PublicKey: []byte("key"), Attestation: webauthn.CredentialAttestation{Object: make([]byte, maxAttestation+1)}},
	}
	for index, item := range invalid {
		if err := ValidateCredential(item); !errors.Is(err, ErrInvalidCredential) {
			t.Fatalf("invalid credential %d = %v", index, err)
		}
	}
	if err := ValidateCredential(&valid); err != nil {
		t.Fatal(err)
	}
}

func TestCredentialCollectionsValidateCloneAndFind(t *testing.T) { //nolint:cyclop // One table covers collection, cloning, and WebAuthn user invariants.
	first, second := credential("first"), credential("second")
	if err := ValidateCredentials([]webauthn.Credential{first, second}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCredentials([]webauthn.Credential{first, first}); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate = %v", err)
	}
	tooMany := make([]webauthn.Credential, MaxCredentials+1)
	if err := ValidateCredentials(tooMany); !errors.Is(err, ErrLimit) {
		t.Fatalf("limit = %v", err)
	}
	invalid := credential("invalid")
	invalid.PublicKey = nil
	if err := ValidateCredentials([]webauthn.Credential{invalid}); !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("invalid list = %v", err)
	}
	if Index([]webauthn.Credential{first, second}, second.ID) != 1 || Index(nil, second.ID) != -1 {
		t.Fatal("credential index is incorrect")
	}
	clone := Clone(first)
	clones := CloneAll([]webauthn.Credential{first})
	clone.ID[0], clone.PublicKey[0], clone.Transport[0], clone.Authenticator.AAGUID[0] = 'X', 'X', protocol.Internal, 9
	clone.Attestation.ClientDataJSON[0], clone.Attestation.ClientDataHash[0] = 'X', 'X'
	clone.Attestation.AuthenticatorData[0], clone.Attestation.Object[0] = 'X', 'X'
	clones[0].ID[0] = 'Y'
	if string(first.ID) != "first" || string(first.PublicKey) != "public-key" || first.Transport[0] != protocol.USB || first.Authenticator.AAGUID[0] != 1 || first.Attestation.ClientDataJSON[0] != 'c' || first.Attestation.ClientDataHash[0] != 'h' || first.Attestation.AuthenticatorData[0] != 'a' || first.Attestation.Object[0] != 'o' {
		t.Fatal("credential clone aliases source data")
	}
	user := NewUser(42, "owner-id", "Owner", []webauthn.Credential{first})
	user.Credentials[0].ID[0] = 'Z'
	if user.Value != 42 || string(user.WebAuthnID()) != "owner-id" || user.WebAuthnName() != "Owner" || user.WebAuthnDisplayName() != "Owner" || len(user.WebAuthnCredentials()) != 1 || string(first.ID) != "first" {
		t.Fatalf("user = %#v", user)
	}
}

func TestCredentialLookupAndRedactedIDs(t *testing.T) {
	for _, input := range []struct{ rawID, handle []byte }{
		{nil, []byte("user")},
		{[]byte("id"), nil},
		{make([]byte, maxCredentialID+1), []byte("user")},
		{[]byte("id"), make([]byte, maxUserHandle+1)},
	} {
		if err := ValidateLookup(input.rawID, input.handle); !errors.Is(err, ErrInvalidCredential) {
			t.Fatalf("lookup = %v", err)
		}
	}
	if err := ValidateLookup([]byte("id"), []byte("user")); err != nil {
		t.Fatal(err)
	}
	id := ID([]byte("credential"))
	if len(id) != 64 || ValidateID(id) != nil {
		t.Fatalf("id = %q", id)
	}
	for _, invalid := range []string{"", id + "0", string(bytes.Repeat([]byte("g"), 64)), "ABCDEF" + id[6:]} {
		if !errors.Is(ValidateID(invalid), ErrInvalidID) {
			t.Fatalf("invalid id accepted: %q", invalid)
		}
	}
}

func TestInventoryAndUpdatePreserveRedactedMetadata(t *testing.T) { //nolint:cyclop // One scenario verifies all redacted inventory fields together.
	first, second := credential("first"), credential("second")
	first.Flags.BackupEligible, first.Flags.BackupState = true, true
	first.Authenticator.SignCount, first.Authenticator.CloneWarning = 7, true
	firstID, secondID := ID(first.ID), ID(second.ID)
	usage := map[string]Usage{firstID: {Tracked: true, LastUsed: 100}, secondID: {Tracked: true, LastUsed: 200}}
	items := Inventory([]webauthn.Credential{first, second}, usage)
	if len(items) != 2 || items[0].Display != firstID[:12] || !items[0].UsageTracked || items[0].LastUsedLabel == "" || items[0].MostRecent || !items[0].BackupEligible || !items[0].BackedUp || !items[0].CloneWarning || items[0].SignCount != 7 || !items[1].MostRecent {
		t.Fatalf("inventory = %#v", items)
	}
	credentials := []webauthn.Credential{first}
	updated := Clone(first)
	updated.Authenticator.SignCount = 9
	if Update(credentials, usage, &updated, time.Unix(300, 0)) != true || credentials[0].Authenticator.SignCount != 9 || usage[firstID].LastUsed != 300 {
		t.Fatalf("update = %#v %#v", credentials, usage)
	}
	if Update(credentials, usage, &updated, time.Unix(250, 0)) != true || usage[firstID].LastUsed != 300 {
		t.Fatal("older use replaced the latest timestamp")
	}
	unknown := credential("unknown")
	if Update(credentials, usage, &unknown, time.Now()) || Update(credentials, nil, &updated, time.Now()) || Update(credentials, usage, nil, time.Now()) {
		t.Fatal("invalid update changed state")
	}
	if got := Inventory(nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("empty inventory = %#v", got)
	}
}
