package supporter

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testActivationID = "00000000-0000-4000-8000-000000000010"

type signingFixture struct {
	private      ed25519.PrivateKey
	public       string
	now          time.Time
	tier         string
	family       string
	activationID string
	collection   bool
	calls        atomic.Int32
	last         activationRequest
}

func newSigningFixture(t *testing.T) *signingFixture {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &signingFixture{
		private: private, public: base64.RawURLEncoding.EncodeToString(public), now: time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC),
		tier: "legacy", family: FamilyPatron, activationID: testActivationID,
	}
}

func (fixture *signingFixture) handler(writer http.ResponseWriter, request *http.Request) {
	fixture.calls.Add(1)
	writer.Header().Set("Content-Type", "application/json")
	fixture.last = activationRequest{}
	if json.NewDecoder(request.Body).Decode(&fixture.last) != nil {
		writer.WriteHeader(http.StatusBadRequest)
		_, _ = writer.Write([]byte(`{"error":"invalid_request"}`))
		return
	}
	sustaining := fixture.family == FamilyLiving
	var expiresAt any
	if sustaining {
		expiresAt = fixture.now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
	}
	value := map[string]any{
		"version": 4, "audience": "com.kinosail.player", "appId": "kino-player", "family": fixture.family, "level": Rank(fixture.tier), "tier": fixture.tier,
		"supporterId": "A1B2C3D4E5", "supportedSince": fixture.now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), "issuedAt": fixture.now.Format(time.RFC3339Nano),
		"expiresAt": expiresAt, "sustaining": sustaining, "founding": true, "installationKey": fixture.last.InstallationKey,
	}
	if fixture.last.RecognitionName != nil {
		value["recognitionName"] = *fixture.last.RecognitionName
	}
	if fixture.collection {
		edition := "2026 Edition"
		if sustaining {
			edition = "Living"
		}
		value["collection"] = map[string]any{"id": completeFleetID, "name": "Complete Fleet", "edition": edition, "appIds": []string{"kino-dashboard", "kino-player"}}
	}
	record, _ := json.Marshal(value)
	_ = json.NewEncoder(writer).Encode(map[string]string{
		"certificate": base64.RawURLEncoding.EncodeToString(record), "signature": base64.RawURLEncoding.EncodeToString(ed25519.Sign(fixture.private, record)),
		"publicKey": fixture.public, "activationId": fixture.activationID,
	})
}

func fixtureService(t *testing.T, fixture *signingFixture) (*Service, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	t.Cleanup(server.Close)
	service, err := New(Config{
		App:           App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", TrackActivations: true, RejectPatronDowngrade: true, GrantPublicKey: true, Legacy: LegacyPlayer},
		ActivationURL: server.URL, SupportURL: "https://support.example/player", HTTPClient: server.Client(), Now: func() time.Time { return fixture.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, server
}

func TestActivationUsesConfiguredPublicKeyStorage(t *testing.T) {
	fixture := newSigningFixture(t)
	grantService, _ := fixtureService(t, fixture)
	grantState, _, err := grantService.Activate(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if err != nil || grantState.PublicKey != "" || grantState.PatronOrder == nil || grantState.PatronOrder.PublicKey != fixture.public {
		t.Fatalf("per-grant key state = %#v, %v", grantState, err)
	}

	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	defer server.Close()
	globalService, err := New(Config{
		App:           App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"},
		ActivationURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return fixture.now },
	})
	if err != nil {
		t.Fatal(err)
	}
	globalState, _, err := globalService.Activate(context.Background(), State{}, ActivationInput{Key: "OTHER_SUPPORTER_KEY"})
	if err != nil || globalState.PublicKey != fixture.public || globalState.PatronOrder == nil || globalState.PatronOrder.PublicKey != "" {
		t.Fatalf("global key state = %#v, %v", globalState, err)
	}
}

func TestActivateAndSaveCommitsOnlyValidatedChangedState(t *testing.T) {
	fixture := newSigningFixture(t)
	service, _ := fixtureService(t, fixture)
	prepared, err := service.Prepare(State{})
	if err != nil {
		t.Fatal(err)
	}
	saves := 0
	save := func(State) error { saves++; return errors.New("disk full") }
	bad, _, activateErr := service.ActivateAndSave(context.Background(), prepared, ActivationInput{Key: "short"}, save)
	if !errors.Is(activateErr, ErrInvalid) || saves != 0 || !reflect.DeepEqual(bad, prepared) {
		t.Fatalf("invalid save boundary = %#v, %d, %v", bad, saves, activateErr)
	}
	next, _, activateErr := service.ActivateAndSave(context.Background(), prepared, ActivationInput{Key: "VALID_SUPPORTER_KEY"}, save)
	if !errors.Is(activateErr, ErrUnavailable) || saves != 1 || !reflect.DeepEqual(next, prepared) {
		t.Fatalf("failed save boundary = %#v, %d, %v", next, saves, activateErr)
	}
}

func TestActivationBuildsBothBadgesAndStatus(t *testing.T) { //nolint:cyclop // One journey covers both badge families and their combined status.
	fixture := newSigningFixture(t)
	fixture.collection = true
	service, _ := fixtureService(t, fixture)
	name := "Quiet Supporter"
	state, status, err := service.Activate(context.Background(), State{}, ActivationInput{Key: "PATRON_SUPPORTER_KEY", RecognitionName: &name})
	if err != nil || status.PatronOrder == nil || status.PatronOrder.Rank != 10 || !status.CompleteFleetActive || state.PatronLevel != 10 {
		t.Fatalf("patron activation = %#v, %#v, %v", state, status, err)
	}
	fixture.family, fixture.tier, fixture.activationID = FamilyLiving, "admiral", "00000000-0000-4000-8000-000000000011"
	state, status, err = service.Activate(context.Background(), state, ActivationInput{Key: "LIVING_SUPPORTER_KEY"})
	if err != nil || status.LivingStandard == nil || !status.LivingStandard.Active || status.LivingStandard.ServiceMonths != 12 ||
		!reflect.DeepEqual(status.LivingStandard.ServiceMarks, []int{3, 6, 12}) || status.BadgeCase.MasterworkLevel != 8 || !status.BadgeCase.MasterworkActive {
		t.Fatalf("living activation = %#v, %#v, %v", state, status, err)
	}
	if fixture.calls.Load() != 2 || len(service.ViewerStatus(state)) != 2 || status.SupportURL != "https://support.example/player" {
		t.Fatalf("calls/status = %d, %#v", fixture.calls.Load(), status)
	}
}

func TestActivationRejectsInputsAndProviderFailuresWithoutStateChange(t *testing.T) {
	fixture := newSigningFixture(t)
	service, server := fixtureService(t, fixture)
	prepared, err := service.Prepare(State{})
	if err != nil {
		t.Fatal(err)
	}
	badName := " padded "
	for _, input := range []ActivationInput{{}, {Key: "short"}, {Key: "VALID_SUPPORTER_KEY", RecognitionName: &badName}} {
		next, _, activateErr := service.Activate(context.Background(), prepared, input)
		if !errors.Is(activateErr, ErrInvalid) || !reflect.DeepEqual(next, prepared) {
			t.Fatalf("Activate(%#v) = %#v, %v", input, next, activateErr)
		}
	}
	if fixture.calls.Load() != 0 {
		t.Fatalf("invalid inputs caused %d calls", fixture.calls.Load())
	}
	server.Close()
	next, _, activateErr := service.Activate(context.Background(), prepared, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if !errors.Is(activateErr, ErrUncertain) || !reflect.DeepEqual(next, prepared) {
		t.Fatalf("failed provider changed state: %#v, %v", next, activateErr)
	}
}

func TestActivationRetainsHigherPatronAndRemembersRejectedReference(t *testing.T) {
	fixture := newSigningFixture(t)
	service, _ := fixtureService(t, fixture)
	state, _, err := service.Activate(context.Background(), State{}, ActivationInput{Key: "HIGH_SUPPORTER_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	original := cloneGrant(state.PatronOrder)
	fixture.tier, fixture.activationID = "friend", "00000000-0000-4000-8000-000000000011"
	next, _, err := service.Activate(context.Background(), state, ActivationInput{Key: "LOW_SUPPORTER_KEY"})
	if !errors.Is(err, ErrConflict) || !reflect.DeepEqual(next.PatronOrder, original) || next.Activations[1].ActivationID != fixture.activationID {
		t.Fatalf("downgrade = %#v, %v", next, err)
	}
	_, _, err = service.Activate(context.Background(), next, ActivationInput{Key: "LOW_SUPPORTER_KEY"})
	if !errors.Is(err, ErrConflict) || fixture.last.ActivationID != fixture.activationID {
		t.Fatalf("retry reference = %#v, %v", fixture.last, err)
	}
}

func TestMalformedSignedCertificateDoesNotChangeState(t *testing.T) {
	fixture := newSigningFixture(t)
	service, _ := fixtureService(t, fixture)
	originalKey := fixture.private
	for name, mutate := range map[string]func(){
		"wrong app":       func() { service.app.ID = "kino-other" },
		"wrong signature": func() { _, fixture.private, _ = ed25519.GenerateKey(rand.Reader) },
		"invalid family":  func() { fixture.family = "patron" },
	} {
		t.Run(name, func(t *testing.T) {
			state := State{}
			mutate()
			next, _, err := service.Activate(context.Background(), state, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
			if !errors.Is(err, ErrInvalid) || !reflect.DeepEqual(next, state) {
				t.Fatalf("result = %#v, %v", next, err)
			}
			service.app.ID, fixture.private, fixture.family = "kino-player", originalKey, FamilyPatron
		})
	}
}

func TestRecoveryAndRotationPreserveOnlySafeReferences(t *testing.T) {
	fixture := newSigningFixture(t)
	service, _ := fixtureService(t, fixture)
	state, _, err := service.Activate(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	state.PatronOrder.Signature = strings.Repeat("A", 86)
	recovered, changed := service.Recover(state)
	if !changed || recovered.PatronOrder != nil || recovered.Activations[0].ActivationID != testActivationID {
		t.Fatalf("recovered = %#v", recovered)
	}
	public, _, _ := ed25519.GenerateKey(rand.Reader)
	rotated, err := service.RotatePublicKey(state, RotationInput{Version: 1, PublicKey: base64.RawURLEncoding.EncodeToString(public), Confirm: true})
	if err != nil || rotated.PatronOrder != nil || rotated.LivingStandard != nil || rotated.InstallationKey != state.InstallationKey {
		t.Fatalf("rotated = %#v, %v", rotated, err)
	}
}
