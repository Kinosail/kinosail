package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestAutomaticMetadataKeepsResolvedEpisodesWhenTVMazeOmitsSomeEpisodes(t *testing.T) {
	var requests [5]atomic.Int32
	provider := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		number, err := strconv.Atoi(request.URL.Query().Get("number"))
		switch {
		case request.URL.Path == "/lookup/shows":
			http.Redirect(writer, request, "/shows/42", http.StatusMovedPermanently)
		case request.URL.Path == "/shows/42":
			_, _ = writer.Write([]byte(`{"id":42,"name":"Example","premiered":"2025-01-01","summary":"<p>A useful show.</p>"}`))
		case request.URL.Path == "/shows/42/episodebynumber" && err == nil && number > 0 && number < len(requests):
			requests[number].Add(1)
			if number == 1 || number == 3 {
				http.NotFound(writer, request)
				return
			}
			_ = json.NewEncoder(writer).Encode(map[string]any{"id": 9000 + number, "name": "Episode " + strconv.Itoa(number), "season": 1, "number": number, "airdate": "2026-01-02", "summary": "<p>Listed plot " + strconv.Itoa(number) + ".</p>"})
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(provider.Close)
	mediaDir, ffprobe := partialShowLibrary(t, 1, 2, 3)
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: ffprobe, Metadata: server.MetadataConfig{TVMazeURL: provider.URL}})
	waitForLibraryText(t, handler, "Listed plot 2.")
	writePartialShowEpisode(t, mediaDir, 4)
	scan := httptest.NewRecorder()
	handler.ServeHTTP(scan, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/scan", nil))
	if scan.Code != http.StatusNoContent {
		t.Fatalf("library scan = %d %q", scan.Code, scan.Body.String())
	}
	waitForLibraryText(t, handler, "Listed plot 4.")
	for _, number := range []int{1, 3} {
		if attempts := requests[number].Load(); attempts != 1 {
			t.Fatalf("unlisted episode %d lookups = %d, want 1 before its backoff expires", number, attempts)
		}
	}
}

func waitForLibraryText(t *testing.T, handler http.Handler, texts ...string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
		missing := ""
		for _, text := range texts {
			if !strings.Contains(response.Body.String(), text) {
				missing = text
			}
		}
		if missing == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("library never showed %q: %d %q", missing, response.Code, response.Body.String())
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func partialShowLibrary(t *testing.T, episodes ...int) (string, string) {
	t.Helper()
	mediaDir := t.TempDir()
	for _, episode := range episodes {
		writePartialShowEpisode(t, mediaDir, episode)
	}
	ffprobe := filepath.Join(t.TempDir(), "ffprobe")
	//nolint:gosec // G306: the fake FFprobe adapter must be executable.
	if err := os.WriteFile(ffprobe, []byte("#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"duration\":\"120\"}}'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	return mediaDir, ffprobe
}

func writePartialShowEpisode(t *testing.T, mediaDir string, episode int) {
	t.Helper()
	season := filepath.Join(mediaDir, "Example", "Season 01")
	if err := os.MkdirAll(season, 0o700); err != nil {
		t.Fatal(err)
	}
	stem := filepath.Join(season, "Example - S01E0"+strconv.Itoa(episode))
	if err := os.WriteFile(stem+".mkv", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(stem+".nfo", []byte(`<episodedetails><uniqueid type="tvdb">123</uniqueid></episodedetails>`), 0o600); err != nil {
		t.Fatal(err)
	}
}
