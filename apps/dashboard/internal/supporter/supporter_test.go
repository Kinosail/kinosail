package supporter

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type memoryStore struct {
	state   State
	found   bool
	saves   int
	saveErr error
}

func (store *memoryStore) LoadJSON(_ context.Context, name string, target any) (bool, error) {
	if name != stateDocument || !store.found {
		return false, nil
	}
	data, _ := json.Marshal(store.state)
	return true, json.Unmarshal(data, target)
}

func (store *memoryStore) SaveJSON(_ context.Context, name string, value any) error {
	if name != stateDocument {
		return ErrInvalid
	}
	store.saves++
	if store.saveErr != nil {
		return store.saveErr
	}
	data, _ := json.Marshal(value)
	store.found = true
	return json.Unmarshal(data, &store.state)
}

func TestActivatePersistsSharedSupporterState(t *testing.T) { //nolint:cyclop // One adapter journey proves request, verification, and persistence.
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			AppID           string `json:"appId"`
			InstallationKey string `json:"installationKey"`
		}
		if json.NewDecoder(request.Body).Decode(&input) != nil || input.AppID != AppID {
			t.Fatal("invalid activation request")
		}
		certificate, _ := json.Marshal(map[string]any{
			"version": 4, "audience": Audience, "appId": AppID, "family": "patron-order", "level": 10, "tier": "legacy",
			"supporterId": dashboardSupporterMark(input.InstallationKey), "supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339),
			"issuedAt": now.Format(time.RFC3339), "expiresAt": nil, "sustaining": false, "founding": true, "installationKey": input.InstallationKey,
		})
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"certificate":  base64.RawURLEncoding.EncodeToString(certificate),
			"signature":    base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, certificate)),
			"publicKey":    base64.RawURLEncoding.EncodeToString(publicKey),
			"activationId": "00000000-0000-4000-8000-000000000010",
		})
	}))
	defer server.Close()
	store := &memoryStore{}
	service, err := New(context.Background(), store, Config{ActivationURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	status, err := service.Activate(context.Background(), ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if err != nil || status.PatronOrder == nil || status.PatronOrder.Rank != 10 {
		t.Fatalf("Activate = %#v, %v", status, err)
	}
	if store.saves != 1 || !store.found || store.state.PatronLevel != 10 || store.state.PatronOrder == nil {
		t.Fatalf("saved state = %#v, saves = %d", store.state, store.saves)
	}
}

func TestActivateRejectsInvalidInputWithoutPersistence(t *testing.T) {
	store := &memoryStore{}
	service, err := New(context.Background(), store, Config{ActivationURL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatal(err)
	}
	badName := " padded "
	for _, input := range []ActivationInput{{}, {Key: "short"}, {Key: "VALID_SUPPORTER_KEY", RecognitionName: &badName}} {
		if _, err = service.Activate(context.Background(), input); !errors.Is(err, ErrInvalid) {
			t.Fatalf("Activate(%#v) error = %v", input, err)
		}
	}
	if store.saves != 0 || store.found {
		t.Fatal("invalid input changed persisted state")
	}
}

func TestActivateDoesNotPublishStateWhenPersistenceFails(t *testing.T) {
	store := &memoryStore{saveErr: errors.New("disk full")}
	service, server := validService(t, store)
	defer server.Close()
	status, err := service.Activate(context.Background(), ActivationInput{Key: "VALID_SUPPORTER_KEY"})
	if !errors.Is(err, ErrUnavailable) || status.Active || service.Status().Active {
		t.Fatalf("Activate = %#v, %v; current = %#v", status, err, service.Status())
	}
}

func TestNewRejectsInvalidPersistedState(t *testing.T) {
	store := &memoryStore{found: true, state: State{InstallationKey: base64.RawURLEncoding.EncodeToString(make([]byte, 32)), LivingLevel: 11}}
	if _, err := New(context.Background(), store, Config{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("New error = %v", err)
	}
}

func validService(t *testing.T, store *memoryStore) (*Service, *httptest.Server) {
	t.Helper()
	now := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	publicKey, privateKey, _ := ed25519.GenerateKey(rand.Reader)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input struct {
			InstallationKey string `json:"installationKey"`
		}
		_ = json.NewDecoder(request.Body).Decode(&input)
		certificate, _ := json.Marshal(map[string]any{
			"version": 4, "audience": Audience, "appId": AppID, "family": "patron-order", "level": 10, "tier": "legacy", "supporterId": dashboardSupporterMark(input.InstallationKey),
			"supportedSince": now.AddDate(-1, 0, 0).Format(time.RFC3339), "issuedAt": now.Format(time.RFC3339), "expiresAt": nil,
			"sustaining": false, "founding": true, "installationKey": input.InstallationKey,
		})
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]string{
			"certificate": base64.RawURLEncoding.EncodeToString(certificate), "signature": base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, certificate)),
			"publicKey": base64.RawURLEncoding.EncodeToString(publicKey), "activationId": "00000000-0000-4000-8000-000000000010",
		})
	}))
	service, err := New(context.Background(), store, Config{ActivationURL: server.URL, HTTPClient: server.Client(), Now: func() time.Time { return now }})
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	return service, server
}

func dashboardSupporterMark(installationKey string) string {
	digest := sha256.Sum256([]byte(installationKey))
	return strings.ToUpper(hex.EncodeToString(digest[:5]))
}
