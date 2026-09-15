package supporter

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func legacyBaseValue(now time.Time, installation string, version int) map[string]any {
	value := map[string]any{
		"version": version, "tier": "admiral", "supporterId": "A1B2C3D4E5", "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano),
		"issuedAt": now.Format(time.RFC3339Nano), "expiresAt": nil, "sustaining": false, "founding": false, "installationKey": installation,
	}
	if version == 2 {
		value["audience"] = "com.kinosail.player"
	}
	return value
}

func TestV1TimeValidationRejectsFutureAndInvalidExpiry(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := testServiceAt(t, App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPlayer}, now)
	value := legacyBaseValue(now, base64.RawURLEncoding.EncodeToString(make([]byte, 32)), 1)
	value["issuedAt"] = now.Add(10 * time.Minute).Format(time.RFC3339Nano)
	record, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var certificate Certificate
	if service.decodeV1Certificate(record, &certificate) {
		t.Fatal("future v1 certificate was accepted")
	}
	certificate = Certificate{SupportedSince: now.Format(time.RFC3339Nano), IssuedAt: now.Format(time.RFC3339Nano), Sustaining: true, ExpiresAt: now.Add(46 * 24 * time.Hour).Format(time.RFC3339Nano)}
	if service.validV1Times(certificate) {
		t.Fatal("overlong v1 expiry was accepted")
	}
}

func TestV2SubscriptionValidationRejectsMismatchAndDefaultsHistoricalWatch(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := testServiceAt(t, App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPlayer}, now)
	value := legacyBaseValue(now, base64.RawURLEncoding.EncodeToString(make([]byte, 32)), 2)
	value["sustaining"], value["expiresAt"], value["subscriptionTier"] = true, now.Add(30*24*time.Hour).Format(time.RFC3339Nano), "unknown"
	record, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var certificate Certificate
	if service.decodeV2Certificate(record, &certificate) {
		t.Fatal("unknown v2 subscription was accepted")
	}
	certificate = Certificate{Sustaining: true, SubscriptionTier: "watch"}
	if validLegacySubscription(map[string]json.RawMessage{"subscriptionTier": json.RawMessage(`"fleet"`)}, &certificate) {
		t.Fatal("mismatched subscription field was accepted")
	}
	certificate.SubscriptionTier = ""
	if !validLegacySubscription(map[string]json.RawMessage{}, &certificate) || certificate.SubscriptionTier != "watch" {
		t.Fatalf("historical subscription default = %q", certificate.SubscriptionTier)
	}
}

func TestV2TimeProfilesRejectInvalidInputAndAcceptSubtitlesExpiry(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	player := testServiceAt(t, App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPlayer}, now)
	if player.validV2Times(Certificate{SupportedSince: "invalid", IssuedAt: now.Format(time.RFC3339Nano)}) {
		t.Fatal("invalid v2 time was accepted")
	}
	subtitles := testServiceAt(t, App{ID: "kino-subtitles", Name: "Kinosail Subtitles", Audience: "com.kinosail.subtitles", MasterworkName: "Perfect Sync", Legacy: LegacySubtitles}, now)
	certificate := Certificate{
		SupportedSince: now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), IssuedAt: now.Format(time.RFC3339Nano),
		ExpiresAt: now.Add(24 * time.Hour).Format(time.RFC3339Nano), Sustaining: true,
	}
	if !subtitles.validV2Times(certificate) {
		t.Fatal("valid subtitles v2 expiry was rejected")
	}
}

func TestLegacyMigrationCanPopulateLivingStandard(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	installation := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	value := legacyBaseValue(now, installation, 2)
	value["sustaining"], value["expiresAt"], value["subscriptionTier"] = true, now.Add(30*24*time.Hour).Format(time.RFC3339Nano), "watch"
	grant := legacySignedGrant(t, value)
	service := testServiceAt(t, App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail", Legacy: LegacyPlayer}, now)
	state := State{InstallationKey: installation, PublicKey: grant.PublicKey, ActivationID: grant.ActivationID, Certificate: grant.Certificate, Signature: grant.Signature, KeyHash: grant.KeyHash}
	migrated := service.migrateLegacy(state)
	if migrated.LivingStandard == nil {
		t.Fatalf("living legacy state was not migrated: %#v", migrated)
	}
}
