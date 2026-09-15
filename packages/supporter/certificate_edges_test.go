package supporter

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func testNow() time.Time {
	return time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
}

func testServiceAt(t *testing.T, app App, now time.Time) *Service {
	t.Helper()
	if app.ID == "" {
		app = App{ID: "kino-player", Name: "Kinosail Player", Audience: "com.kinosail.player", MasterworkName: "Full Sail"}
	}
	service, err := New(Config{App: app, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func currentCertificateValue(now time.Time, installation string) map[string]any {
	return map[string]any{
		"version": 3, "audience": "com.kinosail.player", "appId": "kino-player", "tier": "legacy", "supporterId": "A1B2C3D4E5",
		"supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339Nano), "issuedAt": now.Format(time.RFC3339Nano), "expiresAt": nil,
		"sustaining": false, "founding": true, "installationKey": installation,
	}
}

func TestCertificateVersionAndClassificationEdges(t *testing.T) {
	service := testServiceAt(t, App{}, time.Now().UTC())
	for _, record := range [][]byte{[]byte(`{`), []byte(`{"version":99}`)} {
		if _, _, ok := service.decodeCertificate(record, false); ok {
			t.Fatalf("decodeCertificate(%s) succeeded", record)
		}
	}
	decoded := service.classifyCertificate(Certificate{Sustaining: true, ExpiresAt: "invalid", SubscriptionTier: "watch"}, 2, FamilyPatron)
	if !decoded.valid || decoded.family != FamilyPatron || decoded.certificate.Sustaining || decoded.certificate.ExpiresAt != "" || decoded.certificate.SubscriptionTier != "" {
		t.Fatalf("legacy patron classification = %#v", decoded)
	}
}

func TestCurrentCertificateParserRejectsInvalidVersionAndNullCollection(t *testing.T) {
	var certificate Certificate
	if _, _, ok := parseCurrentCertificate([]byte(`{"version":2}`), &certificate); ok {
		t.Fatal("v2 certificate reached the current parser")
	}
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	value := currentCertificateValue(now, base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	value["collection"] = nil
	record, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := parseCurrentCertificate(record, &certificate); ok {
		t.Fatal("null collection was accepted")
	}
}

func TestCurrentCertificatePredicatesRejectInvalidSignedValues(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := testServiceAt(t, App{}, now)
	if validCurrentExpiry(Certificate{Sustaining: true, ExpiresAt: "2026-09-01T00:00:00Z"}, json.RawMessage(`null`)) {
		t.Fatal("null sustaining expiry was accepted")
	}
	certificate := Certificate{Tier: "legacy", RecognitionName: " padded ", Audience: service.app.Audience, AppID: service.app.ID, SupporterID: "A1B2C3D4E5"}
	if service.validCertificateIdentity(certificate, map[string]json.RawMessage{"recognitionName": json.RawMessage(`" padded "`)}) {
		t.Fatal("invalid recognition name was accepted")
	}
	certificate.RecognitionName = "Visible Name"
	if service.validCertificateIdentity(certificate, map[string]json.RawMessage{}) {
		t.Fatal("unsigned recognition name was accepted")
	}
	if validCertificateTimes(Certificate{SupportedSince: "invalid", IssuedAt: now.Format(time.RFC3339Nano)}, now, true) {
		t.Fatal("invalid certificate time was accepted")
	}
}

func TestCollectionRejectsDuplicateApplicationIDs(t *testing.T) {
	collection := &Collection{ID: completeFleetID, Name: "Complete Fleet", Edition: "2026 Edition", AppIDs: []string{"kino-player", "kino-player"}}
	raw := json.RawMessage(`{"id":"complete-fleet","name":"Complete Fleet","edition":"2026 Edition","appIds":["kino-player","kino-player"]}`)
	if validCollection(collection, raw, false, "kino-player") {
		t.Fatal("duplicate collection application was accepted")
	}
}
