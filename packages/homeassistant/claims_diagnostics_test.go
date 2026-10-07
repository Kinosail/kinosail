package homeassistant

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Isolated log capture protects secret exclusion and rejection correlation that
// browser evidence cannot inspect. State/command effects use registered routes.
func TestRegisteredClaimDiagnosticsAreBoundedAndNeverContainOwnership(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	_, integration, call := claimedHTTPTest(t)
	claim := readClaimReply(t, call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"diagnostic-tab"}`))
	if output.Len() != 0 {
		t.Fatal("successful registration emitted polling diagnostics")
	}
	mux := http.NewServeMux()
	integration.Register(mux, func(next http.Handler) http.Handler { return next }, func(http.ResponseWriter, *http.Request, Approval) error { return nil })
	req := request(http.MethodPut, "/api/v1/home-assistant/players/diagnostic-tab?private=secret-query", strings.NewReader(claimedPlayerJSON))
	req.Header.Set("X-Kinosail-Player-Claim", "wrong_document_secret_0000000000")
	req.Header.Set("Cookie", "private=secret-cookie")
	got := httptest.NewRecorder()
	got.Header().Set("X-Request-ID", "safe-r18-request")
	mux.ServeHTTP(got, req)
	if got.Code != http.StatusForbidden {
		t.Fatalf("ownership rejection = %d", got.Code)
	}
	var entry map[string]any
	if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry) != nil || entry["level"] != "WARN" || entry["operation"] != "state" || entry["failure"] != "ownership" || entry["request_id"] != "safe-r18-request" || entry["target_id"] != "diagnostic-tab" {
		t.Fatal("ownership failure lacks bounded operation/correlation diagnostics")
	}
	for _, secret := range []string{claim.Claim, "wrong_document_secret", "secret-cookie", "secret-query", "Fictional web tab", "position", "?private"} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("diagnostic included private state or ownership")
		}
	}
	output.Reset()
	if got := call(http.MethodPost, "/api/v1/home-assistant/players/claims", `{"id":"diagnostic-tab"}`); got.Code != http.StatusConflict {
		t.Fatalf("live conflict = %d", got.Code)
	}
	if json.Unmarshal(bytes.TrimSpace(output.Bytes()), &entry) != nil || entry["level"] != "DEBUG" || entry["failure"] != "occupied" {
		t.Fatal("routine lease conflict should be quiet at the default log level")
	}
}
