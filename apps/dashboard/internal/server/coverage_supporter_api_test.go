package server

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-dashboard/internal/supporter"
	supporterengine "github.com/MikeO7/kinosail/packages/supporter"
)

func TestCoverageSupporterAPIStatusMapping(t *testing.T) {
	app := newTestApplication(t, Config{})
	session := app.setup(t)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, "/api/v1/supporter/activate", `{"key":"invalid"}`, &session), http.StatusBadRequest)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, "/api/v1/supporter/activate", `{"key":"VALID_SUPPORTER_KEY"}`, &session), http.StatusServiceUnavailable)
	program := coverageSupporterProgram(t)
	app.handler = New(Config{TrustedHosts: []string{"example.com"}}, app.board, app.auth, app.prober, program)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/supporter/activate", strings.NewReader(`{"key":"VALID_SUPPORTER_KEY"}`))
	request.AddCookie(session.cookie)
	request.Header.Set("X-Kinosail-CSRF", session.csrf)
	response := httptest.NewRecorder()
	app.handler.ServeHTTP(response, request)
	assertCoverageAPIStatus(t, response, http.StatusBadGateway)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, "/api/v1/supporter/activate", `{"key":"VALID_SUPPORTER_KEY"}`, &session), http.StatusOK)
	assertCoverageAPIStatus(t, app.request(t, http.MethodPost, "/api/v1/supporter/activate", `{"key":"LOWER_SUPPORTER_KEY"}`, &session), http.StatusConflict)
	if status := program.Status(); status.Rank != 10 || !status.Active {
		t.Fatalf("conflict changed recognition: %+v", status)
	}
}

func coverageSupporterProgram(t *testing.T) *supporter.Service {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input struct{ Key, InstallationKey string }
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Error(err)
			return
		}
		certificate := coverageSupporterCertificate(t, input.Key, input.InstallationKey, now)
		writer.Header().Set("Content-Type", "application/json")
		writeJSON(writer, map[string]string{
			"certificate":  base64.RawURLEncoding.EncodeToString(certificate),
			"signature":    base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, certificate)),
			"publicKey":    base64.RawURLEncoding.EncodeToString(public),
			"activationId": "00000000-0000-4000-8000-000000000010",
		}, http.StatusOK)
	}))
	t.Cleanup(provider.Close)
	_, store, _ := coverageAuthentication(t, false)
	program, err := supporter.New(t.Context(), store, supporter.Config{ActivationURL: provider.URL, HTTPClient: provider.Client(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return program
}

func coverageSupporterCertificate(t *testing.T, key, installation string, now time.Time) []byte {
	t.Helper()
	digest := sha256.Sum256([]byte(installation))
	certificate := supporterengine.Certificate{
		Version: 4, Audience: supporter.Audience, AppID: supporter.AppID, Family: supporterengine.FamilyPatron,
		Level: 10, Tier: "legacy", SupporterID: strings.ToUpper(hex.EncodeToString(digest[:5])),
		SupportedSince: now.AddDate(-1, 0, 0).Format(time.RFC3339), IssuedAt: now.Format(time.RFC3339), InstallationKey: installation,
	}
	if key == "LOWER_SUPPORTER_KEY" {
		certificate.Level = 1
		certificate.Tier = "friend"
	}
	data, err := json.Marshal(struct {
		supporterengine.Certificate
		ExpiresAt *string `json:"expiresAt"`
	}{Certificate: certificate})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
