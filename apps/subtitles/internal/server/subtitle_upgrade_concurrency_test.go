package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleUpgradeKeepsSidecarEditedDuringDownload(t *testing.T) {
	t.Parallel()
	var better atomic.Bool
	edited := "1\n00:00:01,000 --> 00:00:02,000\nEdited by hand\n"
	var target string
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/subtitle.srt" {
			text := "Original"
			if better.Load() {
				text = "Better"
				if err := os.WriteFile(target, []byte(edited), 0o600); err != nil {
					t.Error(err)
				}
			}
			_, _ = writer.Write([]byte("1\n00:00:01,000 --> 00:00:02,000\n" + text + "\n"))
			return
		}
		productionType := 3
		if better.Load() {
			productionType = 0
		}
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"status": true, "results": []any{map[string]any{"name": "Arrival", "type": "movie", "imdb_id": "tt123"}},
			"subtitles": []any{map[string]any{"url": remoteURL(request, "/subtitle.srt"), "language": "EN", "release_name": "Arrival.BluRay-GROUP", "production_type": productionType}},
		})
	}))
	t.Cleanup(remote.Close)
	media := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.BluRay-GROUP.mp4"), ProviderIDs: map[string]string{"imdb": "tt123"}}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	target = subtitleSidecarPath(item, "en")
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), "", sidecarTestIndex(item), nil, "")
	if err := provider.fetchSidecar(t.Context(), item, "en"); err != nil {
		t.Fatal(err)
	}
	key := subtitleRecordKey(item.ID, "en")
	record, _, _ := provider.ledger.record(key)
	record.CheckedAt = time.Now().Add(-48 * time.Hour).Unix()
	if err := provider.ledger.store(key, record); err != nil {
		t.Fatal(err)
	}
	better.Store(true)
	upgraded, err := provider.upgradeSidecar(t.Context(), item, "en")
	current, readErr := os.ReadFile(target)
	if upgraded || err != nil || readErr != nil || string(current) != edited {
		t.Fatalf("upgrade = %v, %v; sidecar = %q, %v", upgraded, err, current, readErr)
	}
	if _, err := os.Stat(target + ".kinosail.bak"); !os.IsNotExist(err) {
		t.Fatalf("upgrade replaced the edited sidecar backup: %v", err)
	}
}
