package playback

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestTraceHTTPDiagnosticsRejectAmbiguityAndProtectLogPrivacy(t *testing.T) { //nolint:cyclop,funlen,gocognit // One HTTP matrix checks strict parsing, valid diagnostics and absence of rejected-input effects.
	const validBody = `{"session":"qa-session-427","event":"error","sequence":1,"detail":"fullscreen:NotAllowedError:playback-retained"}`
	for _, fixture := range []struct {
		name, body string
		status     int
	}{
		{"valid-fullscreen", validBody, http.StatusNoContent},
		{"invalid-error-code", `{"session":"qa-session-427","event":"error","sequence":1,"errorCode":5}`, http.StatusBadRequest},
		{"raw-url", `{"session":"qa-session-427","event":"error","sequence":1,"detail":"https://qa.invalid/private?token=synthetic"}`, http.StatusBadRequest},
		{"oversized-body", strings.Repeat(" ", 4097-len(validBody)) + validBody, http.StatusBadRequest},
		{"duplicate-error-code", `{"session":"qa-session-427","event":"error","sequence":1,"errorCode":5,"errorCode":0}`, http.StatusBadRequest},
		{"case-alias-error-code", `{"session":"qa-session-427","event":"error","sequence":1,"errorCode":5,"ErrorCode":0}`, http.StatusBadRequest},
		{"credential-quality", `{"session":"qa-session-427","event":"error","sequence":1,"quality":"password:qa_synthetic_sensitive_marker"}`, 0},
		{"unknown-fullscreen-name", `{"session":"qa-session-427","event":"error","sequence":1,"detail":"fullscreen:qa_synthetic_sensitive_marker:playback-retained"}`, 0},
		{"credential-detail", `{"session":"qa-session-427","event":"error","sequence":1,"detail":"password:qa_synthetic_sensitive_marker"}`, 0},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var logs bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })
			var sessionSets atomic.Int32
			mux := http.NewServeMux()
			mux.HandleFunc("POST /api/v1/items/{id}/playback-events", TraceHTTP(TraceHTTPConfig{
				Visible:      func(_ *http.Request, id string) bool { return id == "0123456789abcdef" },
				ValidSession: func(value string) bool { return value == "qa-session-427" },
				SetSession:   func(*http.Request, string) { sessionSets.Add(1) },
			}))
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/v1/items/0123456789abcdef/playback-events", strings.NewReader(fixture.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer qa_synthetic_header_marker")
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			logData := logs.String()
			if fixture.status != 0 && response.StatusCode != fixture.status {
				t.Errorf("unexpected HTTP status: got=%d want=%d", response.StatusCode, fixture.status)
			}
			if response.StatusCode == http.StatusBadRequest && (sessionSets.Load() != 0 || logs.Len() != 0) {
				t.Errorf("rejected input caused session/log side effects: sets=%d logBytes=%d", sessionSets.Load(), logs.Len())
			}
			if response.StatusCode == http.StatusNoContent && (sessionSets.Load() != 1 || !strings.Contains(logData, `"level":"WARN"`)) {
				t.Errorf("valid fullscreen fixture did not reach warning logging and session effect")
			}
			if fixture.status == 0 && response.StatusCode != http.StatusNoContent && response.StatusCode != http.StatusBadRequest {
				t.Errorf("credential-marker case neither safely accepted nor rejected: status=%d", response.StatusCode)
			}
			if strings.Contains(logData, "qa_synthetic_sensitive_marker") || strings.Contains(logData, "qa_synthetic_header_marker") {
				t.Errorf("synthetic credential marker entered trace logs")
			}
		})
	}
}
