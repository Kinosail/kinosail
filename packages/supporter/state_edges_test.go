package supporter

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestMatchingActivationRejectsMalformedAndConflictingReferences(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	keyHash := hashKey("VALID_SUPPORTER_KEY")
	state := State{Activations: [activationSlots]Activation{{ActivationID: "invalid", KeyHash: keyHash}}}
	if _, ok := service.matchingActivation(state, keyHash); ok {
		t.Fatal("malformed activation reference matched")
	}
	state = State{
		Activations: [activationSlots]Activation{{ActivationID: testActivationID, KeyHash: keyHash}},
		PatronOrder: &Grant{ActivationID: "00000000-0000-4000-8000-000000000011", KeyHash: keyHash},
	}
	if _, ok := service.matchingActivation(state, keyHash); ok {
		t.Fatal("conflicting grant reference matched")
	}
	state = State{PatronOrder: &Grant{ActivationID: testActivationID, KeyHash: keyHash}}
	if matched, ok := service.matchingActivation(state, keyHash); !ok || matched != testActivationID {
		t.Fatalf("matching grant = %q, %t", matched, ok)
	}
}

func TestRememberActivationEvictsOnlyOldestReference(t *testing.T) {
	activations := [activationSlots]Activation{
		{ActivationID: "00000000-0000-4000-8000-000000000001", KeyHash: "one"},
		{ActivationID: "00000000-0000-4000-8000-000000000002", KeyHash: "two"},
		{ActivationID: "00000000-0000-4000-8000-000000000003", KeyHash: "three"},
		{ActivationID: "00000000-0000-4000-8000-000000000004", KeyHash: "four"},
	}
	next := rememberActivation(activations, Grant{ActivationID: testActivationID, KeyHash: "five"})
	if next[0].KeyHash != "two" || next[3].KeyHash != "five" {
		t.Fatalf("rotated activations = %#v", next)
	}
}

func TestRaiseLevelsIncludesValidLivingGrant(t *testing.T) {
	now := testNow()
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	value := currentCertificateValue(now, installation)
	value["tier"], value["sustaining"], value["expiresAt"] = "admiral", true, now.Add(30*24*time.Hour).Format(time.RFC3339Nano)
	grant := legacySignedGrant(t, value)
	state := State{InstallationKey: installation, LivingStandard: &grant}
	service := testServiceAt(t, App{}, now)
	service.raiseLevels(&state)
	if state.LivingLevel != Rank("admiral") {
		t.Fatalf("living level = %d", state.LivingLevel)
	}
}

func TestRotatePublicKeyRejectsEveryInvalidPrecondition(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	if _, err := service.RotatePublicKey(State{}, RotationInput{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unconfirmed rotation = %v", err)
	}
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(public)
	confirmed := RotationInput{Version: 1, PublicKey: encoded, Confirm: true}
	if _, err = service.RotatePublicKey(State{}, confirmed); !errors.Is(err, ErrConflict) {
		t.Fatalf("unprepared rotation = %v", err)
	}
	state := State{InstallationKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), PublicKey: encoded}
	if _, err = service.RotatePublicKey(state, RotationInput{Version: 1, PublicKey: "invalid", Confirm: true}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed key rotation = %v", err)
	}
	if _, err = service.RotatePublicKey(state, confirmed); !errors.Is(err, ErrInvalid) {
		t.Fatalf("same key rotation = %v", err)
	}
}

func TestEmptyStateIsValid(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	if err := service.ValidateState(State{}); err != nil {
		t.Fatalf("empty state validation = %v", err)
	}
}
