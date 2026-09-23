package supporter

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestActivationRequiresConfiguredIssuerKey(t *testing.T) {
	fixture := newSigningFixture(t)
	fixture.edition = EditionOnce
	_, different, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	wrongKey := base64.RawURLEncoding.EncodeToString(different.Public().(ed25519.PublicKey))
	service, _ := fixtureService(t, fixture, wrongKey)
	saved := 0
	state, _, err := service.ActivateAndSave(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"}, func(State) error { saved++; return nil })
	if err == nil || saved != 0 || !reflect.DeepEqual(state, State{}) {
		t.Fatal("certificate from an unexpected issuer changed state")
	}
	service, _ = fixtureService(t, fixture, fixture.public)
	if state, _, err = service.Activate(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"}); err != nil || state.PatronOrder == nil {
		t.Fatalf("certificate from trusted issuer was rejected: %v", err)
	}
	for _, invalid := range []string{"short", fixture.public + "=", strings.Repeat("A", 65)} {
		if _, err := New(Config{App: service.app, TrustedPublicKey: invalid}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid configured issuer key %q was accepted", invalid)
		}
	}
}

func TestThreeEditionsCoexistInEveryActivationOrder(t *testing.T) { //nolint:cyclop,gocognit // Each permutation checks persistence, downgrade and expiry together.
	for _, order := range [][]string{{EditionOnce, EditionMonthly, EditionYearly}, {EditionOnce, EditionYearly, EditionMonthly}, {EditionMonthly, EditionOnce, EditionYearly}, {EditionMonthly, EditionYearly, EditionOnce}, {EditionYearly, EditionOnce, EditionMonthly}, {EditionYearly, EditionMonthly, EditionOnce}} {
		t.Run(strings.Join(order, "-"), func(t *testing.T) {
			fixture := newSigningFixture(t)
			service, _ := fixtureService(t, fixture)
			state := State{}
			for _, edition := range order {
				fixture.edition = edition
				fixture.family = FamilyLiving
				if edition == EditionOnce {
					fixture.family = FamilyPatron
				}
				next, _, err := service.Activate(context.Background(), state, ActivationInput{Key: "VALID_KEY_" + edition})
				if err != nil {
					t.Fatal(err)
				}
				state = next
			}
			status := service.Status(state)
			if status.PatronOrder == nil || status.Monthly == nil || status.Yearly == nil || status.LivingStandard != nil || len(service.ViewerStatus(state)) != 3 {
				t.Fatalf("missing editions: %+v", status)
			}
			stored, _ := json.Marshal(state)
			var restored State
			if err := json.Unmarshal(stored, &restored); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(service.Status(state), service.Status(restored)) {
				t.Fatal("restart lost collection")
			}
			fixture.tier = "friend"
			fixture.family = FamilyLiving
			fixture.edition = EditionMonthly
			next, status, err := service.Activate(context.Background(), restored, ActivationInput{Key: "VALID_KEY_" + EditionMonthly})
			if err != nil || status.BadgeCase.MonthlyLevel != 10 || status.Yearly.Rank != 10 {
				t.Fatalf("downgrade lost earned tiers: %+v %v", status, err)
			}
			fixture.now = fixture.now.Add(31 * 24 * time.Hour)
			if status = service.Status(next); status.Monthly == nil || !status.Monthly.Expired || status.Yearly == nil || !status.Yearly.Expired || status.BadgeCase.Unlocked != 30 {
				t.Fatal("expiry removed collection")
			}
		})
	}
}

func TestEditionClaimsRejectInvalidAndConflictingValues(t *testing.T) {
	for _, edition := range []string{"unknown", "MONTHLY", " monthly", strings.Repeat("x", 4097), EditionOnce} {
		t.Run(edition[:min(len(edition), 20)], func(t *testing.T) {
			fixture := newSigningFixture(t)
			fixture.edition = edition
			fixture.family = FamilyLiving
			service, _ := fixtureService(t, fixture)
			saved := 0
			state, status, err := service.ActivateAndSave(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"}, func(State) error { saved++; return nil })
			if err == nil || saved != 0 || !reflect.DeepEqual(state, State{}) || status.Active {
				t.Fatal("invalid edition caused a local effect")
			}
		})
	}
}

func TestEditionSaveFailureAndActivePrecedence(t *testing.T) { //nolint:cyclop // Public state transitions include failure and recovery.
	fixture := newSigningFixture(t)
	fixture.edition = EditionMonthly
	fixture.family = FamilyLiving
	service, _ := fixtureService(t, fixture)
	state, _, err := service.Activate(context.Background(), State{}, ActivationInput{Key: "MONTHLY_SUPPORTER"})
	if err != nil {
		t.Fatal(err)
	}
	fixture.now = fixture.now.Add(31 * 24 * time.Hour)
	fixture.edition = EditionYearly
	unchanged, _, err := service.ActivateAndSave(context.Background(), state, ActivationInput{Key: "YEARLY_SUPPORTER"}, func(State) error { return errors.New("disk") })
	if err == nil || !reflect.DeepEqual(unchanged, state) {
		t.Fatal("failed persistence changed collection")
	}
	next, status, err := service.Activate(context.Background(), state, ActivationInput{Key: "YEARLY_SUPPORTER"})
	if err != nil || !status.Active || !status.SubscriptionActive || !status.Monthly.Expired || !status.Yearly.Active {
		t.Fatalf("wrong active projection: %+v %v", status, err)
	}
	for _, edition := range []string{EditionMonthly, EditionYearly} {
		if _, _, ok := service.CertificateForFamily(next, edition); !ok {
			t.Fatal("certificate unavailable", edition)
		}
	}
}

func TestYearlyCertificateCoversPaidYearWithoutWeakeningMonthlyOrLegacyBounds(t *testing.T) { //nolint:cyclop // This table verifies all edition term bounds and persistence effects together.
	fixture := newSigningFixture(t)
	fixture.edition, fixture.family, fixture.expiresIn = EditionYearly, FamilyLiving, 365*24*time.Hour
	service, _ := fixtureService(t, fixture)
	state, status, err := service.Activate(context.Background(), State{}, ActivationInput{Key: "YEARLY_SUPPORTER"})
	if err != nil || !status.Active || status.Yearly == nil {
		t.Fatalf("paid yearly activation = %+v, %v", status, err)
	}
	issuedAt := fixture.now
	fixture.now = issuedAt.Add(46 * 24 * time.Hour)
	if status = service.Status(state); !status.Yearly.Active || status.Yearly.Expired {
		t.Fatal("yearly certificate expired at the monthly refresh boundary")
	}
	fixture.now = issuedAt.Add(365 * 24 * time.Hour)
	if status = service.Status(state); status.Yearly.Active || !status.Yearly.Expired {
		t.Fatal("yearly certificate remained active after its paid period")
	}
	for _, invalid := range []struct {
		edition string
		expiry  time.Duration
	}{
		{EditionYearly, 371 * 24 * time.Hour},
		{EditionMonthly, 46 * 24 * time.Hour},
		{"", 46 * 24 * time.Hour},
	} {
		fixture := newSigningFixture(t)
		fixture.edition, fixture.family, fixture.expiresIn = invalid.edition, FamilyLiving, invalid.expiry
		service, _ := fixtureService(t, fixture)
		saved := 0
		unchanged, _, err := service.ActivateAndSave(context.Background(), State{}, ActivationInput{Key: "INVALID_TERM_KEY"}, func(State) error { saved++; return nil })
		if err == nil || saved != 0 || !reflect.DeepEqual(unchanged, State{}) {
			t.Fatalf("invalid %q term changed state: %v", invalid.edition, err)
		}
	}
}

func TestEditionStateRejectsInvalidBeforeProvider(t *testing.T) {
	fixture := newSigningFixture(t)
	fixture.edition, fixture.family = EditionMonthly, FamilyLiving
	service, _ := fixtureService(t, fixture)
	state, _, err := service.Activate(context.Background(), State{}, ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if err != nil {
		t.Fatal(err)
	}
	cases := []State{state, state, state, state}
	cases[0].MonthlyLevel = -1
	cases[1].YearlyLevel = MaximumLevel + 1
	cases[2].Yearly = state.Monthly
	cases[3].Monthly = cloneGrant(state.Monthly)
	cases[3].Monthly.Signature = "invalid"
	for _, invalid := range cases {
		calls := fixture.calls.Load()
		next, _, err := service.Activate(context.Background(), invalid, ActivationInput{Key: "ANOTHER_VALID_KEY"})
		if err == nil || fixture.calls.Load() != calls || !reflect.DeepEqual(next, invalid) {
			t.Fatal("invalid persisted edition caused a side effect")
		}
	}
}
