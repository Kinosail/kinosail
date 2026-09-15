package supporter

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestLegacyCertificatesUseTheSharedStatusAndCertificateInterface(t *testing.T) { //nolint:cyclop // One table-free compatibility journey covers v1 and v2.
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service, err := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", GrantPublicKey: true, Legacy: LegacyPlayer}, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	patron := legacySignedGrant(t, map[string]any{
		"version": 1, "tier": "commodore", "supporterId": "A1B2C3D4E5", "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano),
		"issuedAt": now.Format(time.RFC3339Nano), "expiresAt": nil, "sustaining": false, "founding": true, "installationKey": installation,
	})
	state := State{InstallationKey: installation, PublicKey: patron.PublicKey, ActivationID: patron.ActivationID, Certificate: patron.Certificate, Signature: patron.Signature, KeyHash: patron.KeyHash}
	status := service.Status(state)
	certificate, badge, found := service.CertificateForFamily(state, FamilyPatron)
	if status.PatronOrder == nil || status.PatronOrder.Rank != 7 || !found || badge.Rank != 7 || certificate.Version != 1 {
		t.Fatalf("v1 status = %#v, %#v, %#v", status, certificate, badge)
	}

	expires := now.Add(30 * 24 * time.Hour).Format(time.RFC3339Nano)
	living := legacySignedGrant(t, map[string]any{
		"version": 2, "audience": "com.kinosail.player", "tier": "admiral", "supporterId": "A1B2C3D4E5",
		"supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), "issuedAt": now.Format(time.RFC3339Nano), "expiresAt": expires,
		"sustaining": true, "subscriptionTier": "watch", "founding": false, "installationKey": installation,
	})
	state = State{InstallationKey: installation, PublicKey: living.PublicKey, LivingStandard: &living}
	status = service.Status(state)
	if status.LivingStandard == nil || status.LivingStandard.Rank != 8 || status.SubscriptionTier != "watch" || !status.SubscriptionActive {
		t.Fatalf("v2 status = %#v", status)
	}
}

func TestMalformedLegacyCertificateAndUnicodeFailClosed(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service, _ := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPlayer}, Now: func() time.Time { return now }})
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	grant := legacySignedRecord(t, []byte(`{"version":3,"audience":"com.kinosail.player","appId":"kino-player","tier":"legacy","supporterId":"A1B2C3D4E5","supportedSince":"2025-08-30T12:00:00Z","issuedAt":"2026-08-30T12:00:00Z","expiresAt":null,"sustaining":false,"founding":true,"installationKey":"`+installation+`","recognitionName":"\ud800"}`))
	state := State{InstallationKey: installation, PublicKey: grant.PublicKey, PatronOrder: &grant}
	if status := service.Status(state); status.PatronOrder != nil {
		t.Fatalf("malformed unicode produced badge %#v", status.PatronOrder)
	}
	grant.Certificate = "not-base64"
	state.PatronOrder = &grant
	if service.ValidateState(state) == nil {
		t.Fatal("malformed stored certificate was accepted")
	}
}

func TestLegacyCertificatesRejectMalformedSignedFieldsAndUnsupportedApps(t *testing.T) { //nolint:cyclop // Negative cases prove signed legacy data fails closed.
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	service, _ := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPlayer}, Now: func() time.Time { return now }})
	base := func() map[string]any {
		return map[string]any{
			"version": 1, "tier": "commodore", "supporterId": "A1B2C3D4E5", "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano),
			"issuedAt": now.Format(time.RFC3339Nano), "expiresAt": nil, "sustaining": false, "founding": true, "installationKey": installation,
		}
	}
	for name, mutate := range map[string]func(map[string]any){
		"null boolean":        func(value map[string]any) { value["sustaining"] = nil },
		"empty expiry":        func(value map[string]any) { value["expiresAt"] = "" },
		"null recognition":    func(value map[string]any) { value["recognitionName"] = nil },
		"unknown field":       func(value map[string]any) { value["unknown"] = true },
		"invalid recognition": func(value map[string]any) { value["recognitionName"] = " padded " },
	} {
		t.Run(name, func(t *testing.T) {
			value := base()
			mutate(value)
			grant := legacySignedGrant(t, value)
			state := State{InstallationKey: installation, PublicKey: grant.PublicKey, PatronOrder: &grant}
			if status := service.Status(state); status.PatronOrder != nil {
				t.Fatalf("malformed legacy certificate produced badge %#v", status.PatronOrder)
			}
		})
	}

	grant := legacySignedGrant(t, base())
	unsupported, _ := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"}, Now: func() time.Time { return now }})
	if status := unsupported.Status(State{InstallationKey: installation, PublicKey: grant.PublicKey, PatronOrder: &grant}); status.PatronOrder != nil {
		t.Fatalf("legacy-disabled app accepted v1 certificate %#v", status.PatronOrder)
	}
}

func TestSubtitlesLegacyProfilePreservesItsV2Rules(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	service, _ := New(Config{App: App{
		ID: "kino-subtitles", Name: "Kinosail Subtitles", Audience: "com.kinosail.subtitles", MasterworkName: "Perfect Sync", Legacy: LegacySubtitles,
	}, Now: func() time.Time { return now }})
	value := map[string]any{
		"version": 2, "audience": "com.kinosail.subtitles", "tier": "friend", "supporterId": "A1B2C3D4E5",
		"supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), "issuedAt": now.Format(time.RFC3339Nano), "expiresAt": nil,
		"sustaining": false, "founding": false, "installationKey": installation, "recognitionName": "Visible Name",
	}
	grant := legacySignedGrant(t, value)
	publicKey := grant.PublicKey
	grant.PublicKey = ""
	state := State{InstallationKey: installation, PublicKey: publicKey, PatronOrder: &grant}
	status := service.Status(state)
	if status.PatronOrder == nil || status.PatronOrder.Rank != 1 || status.PatronOrder.RecognitionName != "" {
		t.Fatalf("valid subtitles v2 status = %#v", status)
	}
	for name, change := range map[string]any{"subscription field": "watch", "null recognition": nil} {
		t.Run(name, func(t *testing.T) {
			copyValue := make(map[string]any, len(value)+1)
			for key, item := range value {
				copyValue[key] = item
			}
			if name == "subscription field" {
				copyValue["subscriptionTier"] = change
			} else {
				copyValue["recognitionName"] = change
			}
			candidate := legacySignedGrant(t, copyValue)
			candidateKey := candidate.PublicKey
			candidate.PublicKey = ""
			if rejected := service.Status(State{InstallationKey: installation, PublicKey: candidateKey, PatronOrder: &candidate}); rejected.PatronOrder != nil {
				t.Fatalf("invalid subtitles v2 produced badge %#v", rejected.PatronOrder)
			}
		})
	}
}

func TestLegacyMigrationPlacesPublicKeyAccordingToAppStorage(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	value := map[string]any{
		"version": 1, "tier": "commodore", "supporterId": "A1B2C3D4E5", "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano),
		"issuedAt": now.Format(time.RFC3339Nano), "expiresAt": nil, "sustaining": false, "founding": true, "installationKey": installation,
	}
	for name, app := range map[string]App{
		"per grant": {ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", GrantPublicKey: true, Legacy: LegacyPlayer},
		"global":    {ID: "kino-subtitles", Name: "Kinosail Subtitles", Audience: "com.kinosail.subtitles", MasterworkName: "Perfect Sync", Legacy: LegacySubtitles},
	} {
		t.Run(name, func(t *testing.T) {
			grant := legacySignedGrant(t, value)
			state := State{InstallationKey: installation, PublicKey: grant.PublicKey, ActivationID: grant.ActivationID, Certificate: grant.Certificate, Signature: grant.Signature, KeyHash: grant.KeyHash}
			service, _ := New(Config{App: app, Now: func() time.Time { return now }})
			migrated := service.migrateLegacy(state)
			if migrated.PatronOrder == nil || app.GrantPublicKey && (migrated.PublicKey != "" || migrated.PatronOrder.PublicKey != grant.PublicKey) ||
				!app.GrantPublicKey && (migrated.PublicKey != grant.PublicKey || migrated.PatronOrder.PublicKey != "") {
				t.Fatalf("migrated state = %#v", migrated)
			}
		})
	}
}

func TestCurrentCertificateRejectsNonPlayerFieldsAndMalformedCollections(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	service, _ := New(Config{App: App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"}, Now: func() time.Time { return now }})
	base := func() map[string]any {
		return map[string]any{
			"version": 4, "audience": "com.kinosail.player", "appId": "kino-player", "family": FamilyPatron, "level": 10, "tier": "legacy",
			"supporterId": "A1B2C3D4E5", "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), "issuedAt": now.Format(time.RFC3339Nano),
			"expiresAt": nil, "sustaining": false, "founding": true, "installationKey": installation,
		}
	}
	for name, mutate := range map[string]func(map[string]any){
		"legacy subscription": func(value map[string]any) { value["subscriptionTier"] = "watch" },
		"collection field": func(value map[string]any) {
			value["collection"] = map[string]any{"id": completeFleetID, "name": "Complete Fleet", "edition": "2026 Edition", "appIds": []string{"kino-dashboard", "kino-player"}, "unknown": true}
		},
		"padded edition": func(value map[string]any) {
			value["collection"] = map[string]any{"id": completeFleetID, "name": "Complete Fleet", "edition": "2026 Edition ", "appIds": []string{"kino-dashboard", "kino-player"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := base()
			mutate(value)
			grant := legacySignedGrant(t, value)
			state := State{InstallationKey: installation, PublicKey: grant.PublicKey, PatronOrder: &grant}
			if status := service.Status(state); status.PatronOrder != nil {
				t.Fatalf("invalid current certificate produced badge %#v", status.PatronOrder)
			}
		})
	}
	record := []byte(fmt.Sprintf(`{"version":4,"audience":"com.kinosail.player","appId":"kino-player","family":"patron-order","level":10,"tier":"legacy","supporterId":"A1B2C3D4E5","supportedSince":"2025-08-30T12:00:00Z","issuedAt":"2026-08-30T12:00:00Z","expiresAt":null,"sustaining":false,"founding":true,"installationKey":"%s","collection":{"id":"complete-fleet","id":"complete-fleet","name":"Complete Fleet","edition":"2026 Edition","appIds":["kino-dashboard","kino-player"]}}`, installation))
	grant := legacySignedRecord(t, record)
	if status := service.Status(State{InstallationKey: installation, PublicKey: grant.PublicKey, PatronOrder: &grant}); status.PatronOrder != nil {
		t.Fatalf("duplicate collection field produced badge %#v", status.PatronOrder)
	}
}

func TestDerivativeSupporterIDBindingFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	service, _ := New(Config{App: App{
		ID: "kino-example", Name: "Example App", Audience: "com.example.app", MasterworkName: "Example Masterwork", BindSupporterIDToInstallation: true,
	}, Now: func() time.Time { return now }})
	value := map[string]any{
		"version": 4, "audience": "com.example.app", "appId": "kino-example", "family": FamilyPatron, "level": 10, "tier": "legacy",
		"supporterId": "A1B2C3D4E5", "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), "issuedAt": now.Format(time.RFC3339Nano),
		"expiresAt": nil, "sustaining": false, "founding": true, "installationKey": installation,
	}
	grant := legacySignedGrant(t, value)
	state := State{InstallationKey: installation, PublicKey: grant.PublicKey, PatronOrder: &grant}
	if status := service.Status(state); status.PatronOrder != nil {
		t.Fatalf("unbound supporter id produced badge %#v", status.PatronOrder)
	}
	value["supporterId"] = supporterMark(installation)
	grant = legacySignedGrant(t, value)
	state.PublicKey, state.PatronOrder = grant.PublicKey, &grant
	if status := service.Status(state); status.PatronOrder == nil {
		t.Fatal("installation-bound supporter id was rejected")
	}
}

func legacySignedGrant(t *testing.T, value map[string]any) Grant {
	t.Helper()
	record, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return legacySignedRecord(t, record)
}

func legacySignedRecord(t *testing.T, record []byte) Grant {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return Grant{
		ActivationID: testActivationID, Certificate: base64.RawURLEncoding.EncodeToString(record),
		Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, record)), PublicKey: base64.RawURLEncoding.EncodeToString(public),
		KeyHash: base64.RawURLEncoding.EncodeToString(make([]byte, 32)),
	}
}
