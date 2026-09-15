package supporter

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"reflect"
	"testing"
	"time"
)

func TestServiceDefaultsAndPreparationFailurePaths(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	if service.client.Timeout != 10*time.Second || service.client.CheckRedirect(nil, nil) == nil {
		t.Fatal("safe HTTP client defaults changed")
	}
	invalid := State{InstallationKey: "invalid"}
	if next, err := service.Prepare(invalid); !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(next, invalid) {
		t.Fatalf("Prepare(invalid) = %#v, %v", next, err)
	}
}

func TestActivationPreconditionsCauseNoNetworkOrStateChange(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	state := State{}
	next, _, err := service.Activate(context.Background(), state, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if !errors.Is(err, ErrUnavailable) || !reflect.DeepEqual(next, state) {
		t.Fatalf("unconfigured activation = %#v, %v", next, err)
	}
	if _, _, _, ok := service.prepareActivation(State{LivingLevel: MaximumLevel + 1}, "VALID_SUPPORTER_KEY"); ok {
		t.Fatal("out-of-range saved level was prepared")
	}
	if _, _, _, ok := service.prepareActivation(State{InstallationKey: "invalid"}, "VALID_SUPPORTER_KEY"); ok {
		t.Fatal("malformed installation was prepared")
	}
	service.endpoint = "https://activation.example"
	malformed := State{InstallationKey: "invalid"}
	next, _, err = service.Activate(context.Background(), malformed, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(next, malformed) {
		t.Fatalf("malformed saved activation = %#v, %v", next, err)
	}
}

func TestActivationOutputRejectsIdentityBeforeChangingState(t *testing.T) {
	service := testServiceAt(t, App{}, testNow())
	state, err := service.Prepare(State{})
	if err != nil {
		t.Fatal(err)
	}
	next, grant, decoded, validationErr := service.validateActivationOutput(state, state, activationResponse{ActivationID: "invalid"}, "", hashKey("VALID_SUPPORTER_KEY"), nil)
	if !errors.Is(validationErr, ErrInvalid) || grant != nil || decoded.valid || !reflect.DeepEqual(next, state) {
		t.Fatalf("invalid output = %#v, %#v, %#v, %v", next, grant, decoded, validationErr)
	}
	if acceptsPublicKey(State{PublicKey: "saved"}, "different") {
		t.Fatal("mismatched public key was accepted")
	}
}

func TestNonRejectingAppKeepsHigherPatronWithoutMutation(t *testing.T) {
	now := testNow()
	service := testServiceAt(t, App{}, now)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	value := currentCertificateValue(now, installation)
	existing := legacySignedGrant(t, value)
	state := State{InstallationKey: installation, PatronOrder: &existing, PatronLevel: MaximumLevel}
	grant := Grant{ActivationID: "00000000-0000-4000-8000-000000000011"}
	decoded := decodedGrant{certificate: Certificate{Tier: "friend"}, family: FamilyPatron, valid: true}
	next, _, err := service.applyActivation(state, cloneState(state), &grant, decoded)
	if err != nil || !reflect.DeepEqual(next, state) {
		t.Fatalf("non-rejecting downgrade = %#v, %v", next, err)
	}
}

func TestActivateAndSaveReturnsCommittedState(t *testing.T) {
	fixture := newSigningFixture(t)
	service, _ := fixtureService(t, fixture)
	saves := 0
	next, status, err := service.ActivateAndSave(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"}, func(saved State) error {
		saves++
		if saved.PatronOrder == nil {
			t.Fatal("save received no patron grant")
		}
		return nil
	})
	if err != nil || saves != 1 || next.PatronOrder == nil || status.PatronOrder == nil {
		t.Fatalf("ActivateAndSave = %#v, %#v, %d, %v", next, status, saves, err)
	}
}

func TestWrappedErrorPresentationAndCustomClientTimeout(t *testing.T) {
	if got := invalid("specific failure").Error(); got != "specific failure" {
		t.Fatalf("wrapped error = %q", got)
	}
	client := &http.Client{Timeout: time.Second}
	service, err := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"}, HTTPClient: client})
	if err != nil || service.client.Timeout != time.Second {
		t.Fatalf("custom client = %#v, %v", service, err)
	}
}
