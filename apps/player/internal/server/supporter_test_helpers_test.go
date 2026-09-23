package server_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

const supporterActivationID = "00000000-0000-4000-8000-000000000010"

type supporterSigner struct {
	private         ed25519.PrivateKey
	public          string
	now             time.Time
	tier            string
	audience        string
	appID           string
	recognitionName string
	sustaining      bool
	expiresAt       any
	supportedSince  time.Time
	collection      any
	activationID    string
	edition         string
	version         int
	omitField       string
	extraField      bool
	calls           atomic.Int32
	last            map[string]string
}

func newSupporterSigner(t *testing.T) *supporterSigner {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	return &supporterSigner{private: private, public: base64.RawURLEncoding.EncodeToString(public), now: now, tier: "legacy", audience: "com.kinosail.player", appID: "kino-player", sustaining: true, expiresAt: now.Add(45 * 24 * time.Hour).Format(time.RFC3339Nano), supportedSince: now.AddDate(-1, -2, 0), activationID: supporterActivationID, version: 4}
}

func (signer *supporterSigner) handler(writer http.ResponseWriter, request *http.Request) {
	signer.calls.Add(1)
	writer.Header().Set("Content-Type", "application/json")
	signer.last = nil
	if err := json.NewDecoder(request.Body).Decode(&signer.last); err != nil {
		http.Error(writer, "invalid", http.StatusBadRequest)
		return
	}
	certificate := signer.certificate()
	record, _ := json.Marshal(certificate)
	_ = json.NewEncoder(writer).Encode(map[string]string{
		"certificate": base64.RawURLEncoding.EncodeToString(record), "signature": base64.RawURLEncoding.EncodeToString(ed25519.Sign(signer.private, record)),
		"publicKey": signer.public, "activationId": signer.activationID,
	})
}

func (signer *supporterSigner) certificate() map[string]any {
	name := signer.recognitionName
	if requested := signer.last["recognitionName"]; requested != "" && (signer.tier == "commodore" || signer.tier == "admiral" || signer.tier == "northstar" || signer.tier == "legacy") {
		name = requested
	}
	certificate := map[string]any{
		"version": signer.version, "audience": signer.audience, "appId": signer.appID, "tier": signer.tier, "supporterId": "A1B2C3D4E5", "supportedSince": signer.supportedSince.Format(time.RFC3339Nano),
		"issuedAt": signer.now.Format(time.RFC3339Nano), "expiresAt": signer.expiresAt, "sustaining": signer.sustaining, "founding": true, "installationKey": signer.last["installationKey"],
	}
	if signer.version >= 4 {
		certificate["family"] = map[bool]string{false: "patron-order", true: "living-standard"}[signer.sustaining]
		certificate["level"] = map[string]int{"friend": 1, "crew": 2, "navigator": 3, "patron": 4, "steward": 5, "lighthouse": 6, "commodore": 7, "admiral": 8, "northstar": 9, "legacy": 10}[signer.tier]
	}
	if signer.version == 5 {
		certificate["edition"] = signer.edition
	}
	if name != "" {
		certificate["recognitionName"] = name
	}
	if signer.collection != nil {
		certificate["collection"] = signer.collection
	}
	if signer.extraField {
		certificate["paymentId"] = "must-not-pass"
	}
	delete(certificate, signer.omitField)
	return certificate
}

func assertLivingActivationRequest(t *testing.T, request map[string]string) {
	t.Helper()
	if request["key"] != "LIVING_STANDARD_KEY" || request["appId"] != "kino-player" || request["recognitionName"] != "Private Patron" || len(request["installationKey"]) != 43 || len(request) != 4 {
		t.Fatalf("living activation request = %#v", request)
	}
}

func assertSupporterCertificatePrivacy(t *testing.T, installationKey string, responses ...*httptest.ResponseRecorder) {
	t.Helper()
	for _, response := range responses {
		if response.Header().Get("Content-Type") != "image/svg+xml; charset=utf-8" || response.Header().Get("Cache-Control") != "private, no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("certificate headers = %v", response.Header())
		}
		for _, secret := range []string{"A1B2C3D4E5", "LIVING_STANDARD_KEY", "PATRON_ORDER_KEY", installationKey, supporterActivationID, "must-not-pass"} {
			if secret != "" && strings.Contains(response.Body.String(), secret) {
				t.Fatalf("certificate exposed private value %q", secret)
			}
		}
	}
}

func supporterServer(t *testing.T, dataDir string, signer *supporterSigner, upstream *httptest.Server) (http.Handler, string) {
	return servertest.SupporterServer(t, dataDir, upstream, func() time.Time { return signer.now }, func(data, activation, support string, client *http.Client, now func() time.Time) http.Handler {
		return server.New(server.Config{DataDir: data, RequireAuth: true, Supporter: server.SupporterConfig{ActivationURL: activation, SupportURL: support, HTTPClient: client, Now: now}})
	}, testTOTP)
}
