package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func subDLFailureFixture(t *testing.T, subtitles []map[string]any, files map[string]int) (*httptest.Server, library.Item, string) {
	t.Helper()
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if status, found := files[request.URL.Path]; found {
			if status != http.StatusOK {
				writer.WriteHeader(status)
				return
			}
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\nWorking link\n"))
			return
		}
		for _, subtitle := range subtitles {
			subtitle["url"] = remoteURL(request, subtitle["path"].(string))
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie", "imdb_id": "tt123"}}, "subtitles": subtitles,
		})
	}))
	t.Cleanup(remote.Close)
	media, data := t.TempDir(), t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), ProviderIDs: map[string]string{"imdb": "tt123"}}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	return remote, item, data
}

func TestOneDeadSubDLLinkDoesNotBlockTheNextCandidate(t *testing.T) {
	t.Parallel()
	subtitles := []map[string]any{
		{"path": "/missing.srt", "language": "EN", "release_name": "Arrival.BluRay-GROUP", "production_type": 0},
		{"path": "/working.srt", "language": "EN", "release_name": "Arrival.BluRay-GROUP", "production_type": 0},
	}
	remote, item, data := subDLFailureFixture(t, subtitles, map[string]int{"/missing.srt": http.StatusNotFound, "/working.srt": http.StatusOK})
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), data, sidecarTestIndex(item), nil, "", testStateDatabase(t, data))

	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatalf("fetch after one dead link = %v", err)
	}
	installed, err := os.ReadFile(filepath.Join(filepath.Dir(item.Path), "Arrival.BluRay-GROUP.en.srt"))
	if err != nil || !strings.Contains(string(installed), "Working link") {
		t.Fatalf("installed subtitle = %q, %v", installed, err)
	}
	if err = provider.health.before("SubDL"); err != nil {
		t.Fatalf("a dead file link delayed the whole provider: %v", err)
	}
}

func TestPartialProviderOutageGetsTheShortProviderErrorBackoff(t *testing.T) {
	t.Parallel()
	remote, item, data := subDLFailureFixture(t, []map[string]any{}, nil)
	outage := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(outage.Close)
	config := SubtitleConfig{URL: remote.URL, APIKey: "key", OpenSubtitles: OpenSubtitlesConfig{URL: outage.URL, APIKey: "key", Username: "user", Password: "password"}}
	provider := newSubtitleProvider(config, t.TempDir(), data, sidecarTestIndex(item), nil, "", testStateDatabase(t, data))

	if err := provider.fetchSidecar(t.Context(), item, "en"); err == nil {
		t.Fatal("fetch with no candidates succeeded")
	}
	record, found, err := provider.ledger.search(subtitleSearchKey(item.ID, "en", provider.preference()))
	if err != nil || !found || record.Outcome != "provider-error" {
		t.Fatalf("partial outage recorded as %#v (found=%t, err=%v)", record, found, err)
	}
	if retry := time.Unix(record.NextAt, 0).Sub(time.Unix(record.CheckedAt, 0)); retry > time.Hour {
		t.Fatalf("partial outage retry waits %v", retry)
	}
}
