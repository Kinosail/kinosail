package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestTVMazeFallbackSharesMetadataThroughWebAndAPI(t *testing.T) {
	provider := httptest.NewServer(http.HandlerFunc(serveTVMazeMetadataFixture))
	t.Cleanup(provider.Close)
	mediaDir, ffprobe := tvMazeMetadataLibrary(t)
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: ffprobe, Metadata: server.MetadataConfig{TVMazeURL: provider.URL}})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var catalog struct {
		Items []struct {
			ID      string `json:"id"`
			Episode int    `json:"episode"`
			Plot    string `json:"plot"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &catalog); err != nil || len(catalog.Items) != 2 {
		t.Fatalf("library = %d %q error=%v", response.Code, response.Body.String(), err)
	}
	ids := make(map[int]string, len(catalog.Items))
	for _, item := range catalog.Items {
		ids[item.Episode] = item.ID
	}
	refresh := httptest.NewRecorder()
	handler.ServeHTTP(refresh, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/metadata", nil))
	if refresh.Code != http.StatusNoContent {
		t.Fatalf("metadata refresh = %d %q", refresh.Code, refresh.Body.String())
	}
	listed := httptest.NewRecorder()
	handler.ServeHTTP(listed, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	if !strings.Contains(listed.Body.String(), "From TVmaze.") || !strings.Contains(listed.Body.String(), "Keep this local.") || strings.Contains(listed.Body.String(), "Remote override.") {
		t.Fatalf("API metadata = %q", listed.Body.String())
	}
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+ids[2], nil))
	if player.Code != http.StatusOK || !strings.Contains(player.Body.String(), "From TVmaze.") {
		t.Fatalf("web metadata = %d %q", player.Code, player.Body.String())
	}
}

func tvMazeMetadataLibrary(t *testing.T) (string, string) {
	t.Helper()
	mediaDir := t.TempDir()
	season := filepath.Join(mediaDir, "Example", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, number := range []string{"01", "02"} {
		stem := filepath.Join(season, "Example - S01E"+number)
		if err := os.WriteFile(stem+".mkv", nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(stem+".nfo", []byte(`<episodedetails><uniqueid type="tvdb">123</uniqueid>`+map[string]string{"01": `<plot>Keep this local.</plot>`, "02": ""}[number]+`</episodedetails>`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ffprobe := filepath.Join(t.TempDir(), "ffprobe")
	//nolint:gosec // G306: the fake FFprobe adapter must be executable.
	if err := os.WriteFile(ffprobe, []byte("#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"duration\":\"120\"}}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return mediaDir, ffprobe
}

func serveTVMazeMetadataFixture(writer http.ResponseWriter, request *http.Request) {
	switch request.URL.Path {
	case "/lookup/shows":
		http.Redirect(writer, request, "/shows/42", http.StatusMovedPermanently)
	case "/shows/42":
		_, _ = writer.Write([]byte(`{"id":42,"name":"Example","premiered":"2025-01-01","summary":"<p>A useful show.</p>"}`))
	case "/shows/42/episodebynumber":
		number := request.URL.Query().Get("number")
		episode := map[string]int{"1": 1, "2": 2}[number]
		name := map[string]string{"1": "The Return", "2": "The Signal"}[number]
		plot := map[string]string{"1": "Remote override.", "2": "From TVmaze."}[number]
		_ = json.NewEncoder(writer).Encode(map[string]any{"id": 9000 + episode, "name": name, "season": 1, "number": episode, "airdate": "2026-01-02", "summary": "<p>" + plot + "</p>"})
	default:
		http.NotFound(writer, request)
	}
}
