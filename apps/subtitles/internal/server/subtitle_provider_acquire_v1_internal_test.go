package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleAcquireSkipsRejectedAndInvalidCandidates(t *testing.T) { //nolint:cyclop,funlen // One provider fixture proves every candidate rejection stage.
	t.Parallel()
	mode := "valid"
	var modeMu sync.RWMutex
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		modeMu.RLock()
		current := mode
		modeMu.RUnlock()
		switch request.URL.Path {
		case "/api/v1/subtitles":
			writer.Header().Set("Content-Type", "application/json")
			release := "Movie.2026.BluRay-GROUP"
			if current == "low match" {
				release = "Different Edition"
			}
			_ = json.NewEncoder(writer).Encode(subDLResponse{
				Status:  true,
				Results: []subDLResult{{Type: "movie", Name: "Movie", Year: 2026}},
				Subtitles: []subDLSubtitle{{
					ReleaseName: release, Language: "en", URL: "/candidate.srt",
				}},
			})
		case "/candidate.srt":
			switch current {
			case "valid", "low match":
				_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nDialogue\n"))
			case "malformed":
				_, _ = writer.Write([]byte("1\nbad --> bad\nDialogue\n"))
			default:
				http.Error(writer, "unavailable", http.StatusServiceUnavailable)
			}
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(remote.Close)
	item := library.Item{Kind: "video", ID: "0123456789abcdef", Title: "Movie", Year: "2026", Path: filepath.Join(t.TempDir(), "Movie.2026.BluRay-GROUP.mp4")}
	newProvider := func() *subtitleProvider {
		return newSubtitleProvider(SubtitleConfig{URL: remote.URL + "/api/v1", APIKey: "key"}, t.TempDir(), t.TempDir(), nil, nil, "")
	}
	cleaned, record, err := newProvider().acquire(t.Context(), item, "en", nil)
	if err != nil || record.Source != "subdl" || record.Synchronization != "none" || len(cleaned.Data) == 0 {
		t.Fatalf("valid candidate = %#v, %#v, %v", cleaned, record, err)
	}
	if _, _, err = newProvider().acquire(t.Context(), item, "en", func(subtitleDownloadCandidate) bool { return false }); !errors.Is(err, errNoTrustedSubtitle) {
		t.Fatalf("rejected candidate = %v", err)
	}
	modeMu.Lock()
	mode = "malformed"
	modeMu.Unlock()
	if _, _, err = newProvider().acquire(t.Context(), item, "en", nil); !errors.Is(err, errNoTrustedSubtitle) {
		t.Fatalf("malformed candidate = %v", err)
	}
	modeMu.Lock()
	mode = "failed"
	modeMu.Unlock()
	if _, _, err = newProvider().acquire(t.Context(), item, "en", nil); !errors.Is(err, errNoTrustedSubtitle) {
		t.Fatalf("failed candidate = %v", err)
	}
	modeMu.Lock()
	mode = "low match"
	modeMu.Unlock()
	if _, _, err = newProvider().acquire(t.Context(), item, "en", nil); !errors.Is(err, errNoTrustedSubtitle) {
		t.Fatalf("low-confidence candidate = %v", err)
	}
}

func TestSubtitleSidecarRejectsInvalidExistingAndUnavailableRequests(t *testing.T) {
	t.Parallel()
	item := library.Item{Kind: "video", ID: "0123456789abcdef", Title: "Movie", Path: filepath.Join(t.TempDir(), "Movie.mp4")}
	provider := newSubtitleProvider(SubtitleConfig{}, t.TempDir(), t.TempDir(), nil, nil, "")
	if provider.fetchSidecar(t.Context(), library.Item{Kind: "audio"}, "en") == nil || provider.fetchSidecar(t.Context(), item, "english") == nil {
		t.Fatal("invalid sidecar request was accepted")
	}
	target := subtitleSidecarPath(item, "en")
	if err := os.WriteFile(target, []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := provider.fetchSidecar(t.Context(), item, "en"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing sidecar = %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := provider.fetchSidecar(t.Context(), item, "en"); err == nil {
		t.Fatal("unconfigured sidecar search was accepted")
	}
}
