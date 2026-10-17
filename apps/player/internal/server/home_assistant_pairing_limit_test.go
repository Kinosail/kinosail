package server_test

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHomeAssistantPairingLimitsGuessesPerClientAndPerCode(t *testing.T) {
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	handler, owner := apiServer(t)
	assertAPIBody(t, apiCall(t, handler, owner, http.MethodPut, "/api/v1/settings/home-assistant", map[string]any{"enabled": true}), http.StatusOK, `"status":"saved"`)
	keys := apiCall(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil).Body.String()

	first := offerHomeAssistantPairing(t, handler, owner)
	for guess := range 10 {
		assertPairAttempt(t, handler, "192.0.2.50", wrongPairingCode(first, guess), http.StatusBadRequest)
	}
	limited := assertPairAttempt(t, handler, "192.0.2.50", wrongPairingCode(first, 10), http.StatusTooManyRequests)
	if limited.Header().Get("Retry-After") != "60" {
		t.Fatalf("limited pairing headers = %v", limited.Header())
	}
	assertPairAttempt(t, handler, "192.0.2.50", first, http.StatusTooManyRequests)
	if after := apiCall(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil).Body.String(); after != keys {
		t.Fatalf("limited pairing changed keys\nbefore: %s\nafter: %s", keys, after)
	}
	assertPairAttempt(t, handler, "192.0.2.51", first, http.StatusCreated)

	keys = apiCall(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil).Body.String()
	second := offerHomeAssistantPairing(t, handler, owner)
	logs.Reset()
	for guess := range 100 {
		assertPairAttempt(t, handler, fmt.Sprintf("192.0.2.%d", 100+guess/10), wrongPairingCode(second, guess), http.StatusBadRequest)
	}
	assertPairAttempt(t, handler, "192.0.2.200", second, http.StatusBadRequest)
	if after := apiCall(t, handler, owner, http.MethodGet, "/api/v1/api-keys", nil).Body.String(); after != keys {
		t.Fatalf("invalidated pairing code created a key\nbefore: %s\nafter: %s", keys, after)
	}
	invalidated := pairingInvalidationLog(t, logs.String())
	for _, want := range []string{`"level":"WARN"`, `"outcome":"codes_invalidated"`, `"failures":100`, `"codes":1`} {
		if !strings.Contains(invalidated, want) {
			t.Fatalf("pairing invalidation log %q lacks %s", invalidated, want)
		}
	}
	if strings.Contains(logs.String(), second) {
		t.Fatal("pairing logs included the pairing code")
	}
	assertPairAttempt(t, handler, "192.0.2.201", offerHomeAssistantPairing(t, handler, owner), http.StatusCreated)
}

func offerHomeAssistantPairing(t *testing.T, handler http.Handler, owner string) string {
	t.Helper()
	pairing := apiCall(t, handler, owner, http.MethodPost, "/api/v1/home-assistant/pairings", map[string]any{})
	var offered struct {
		Code string `json:"code"`
	}
	mustJSON(t, pairing, &offered)
	if pairing.Code != http.StatusCreated || len(offered.Code) != 8 {
		t.Fatalf("pairing offer = %d %#v", pairing.Code, offered)
	}
	return offered.Code
}

func wrongPairingCode(offered string, guess int) string {
	code := fmt.Sprintf("%08d", guess)
	if code == offered {
		return fmt.Sprintf("%08d", 99999999-guess)
	}
	return code
}

func assertPairAttempt(t *testing.T, handler http.Handler, remote, code string, want int) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/home-assistant/pair", strings.NewReader(`{"code":"`+code+`","name":"Home"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = remote + ":1234"
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != want {
		t.Fatalf("pairing from %s = %d %q; want %d", remote, response.Code, response.Body.String(), want)
	}
	return response
}

func pairingInvalidationLog(t *testing.T, logs string) string {
	t.Helper()
	for line := range strings.SplitSeq(logs, "\n") {
		if strings.Contains(line, "Home Assistant pairing") {
			return line
		}
	}
	t.Fatalf("no pairing invalidation log in %q", logs)
	return ""
}
