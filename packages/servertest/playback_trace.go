package servertest

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// PlaybackTraceRejectsInvalidInputWithoutLogging verifies validation precedes telemetry side effects.
func (fixture LibraryAPIFixture) PlaybackTraceRejectsInvalidInputWithoutLogging(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	for name, body := range map[string]string{
		"missing session":  `{"event":"play"}`,
		"invalid session":  `{"session":"../../private","event":"play"}`,
		"unknown event":    `{"session":"trace-session","event":"private-title"}`,
		"unknown field":    `{"session":"trace-session","event":"play","url":"/private/media"}`,
		"bad state":        `{"session":"trace-session","event":"play","readyState":5}`,
		"oversized detail": `{"session":"trace-session","event":"play","detail":"` + strings.Repeat("x", 65) + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			previous := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
			defer slog.SetDefault(previous)
			request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/items/"+id+"/playback-events", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("invalid trace = %d %q", response.Code, response.Body.String())
			}
			if strings.Contains(output.String(), `"msg":"playback trace"`) {
				t.Fatalf("invalid trace was logged: %s", output.String())
			}
		})
	}
}
