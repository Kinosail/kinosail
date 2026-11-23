package server_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestStartupFailureLogsStageWithoutPrivatePaths(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	cache := filepath.Join(t.TempDir(), "Private Household Cache")
	downloads := filepath.Join(cache, "downloads")
	if err := os.MkdirAll(downloads, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(downloads, "aaaaaaaaaaaaaaaa.json"), []byte(`{"id":"aaaaaaaaaaaaaaaa","extension":"../../outside"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{Lifecycle: t.Context(), DataDir: t.TempDir(), CacheDir: cache})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("startup = %d", response.Code)
	}
	var failure map[string]any
	for line := range strings.Lines(output.String()) {
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) == nil && entry["operation"] == "startup" {
			failure = entry
		}
	}
	if failure["level"] != "ERROR" || failure["stage"] != "downloads" || failure["failure_class"] == nil || failure["outcome"] != "unavailable" {
		t.Fatalf("startup failure log = %v\n%s", failure, output.String())
	}
	if strings.Contains(output.String(), "Private Household") || strings.Contains(output.String(), "outside") {
		t.Fatalf("startup failure log leaked private paths:\n%s", output.String())
	}
}
