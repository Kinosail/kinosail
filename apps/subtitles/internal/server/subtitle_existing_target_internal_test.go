package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestFetchIntoAnOccupiedSidecarBacksOffInsteadOfRetryingEveryRun(t *testing.T) {
	t.Parallel()
	var requests atomic.Int64
	remote := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		writer.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(remote.Close)
	media, data := t.TempDir(), t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Title: "Arrival", Path: filepath.Join(media, "Arrival.mp4")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(media, "Arrival.en.srt"), []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := newSubtitleProvider(SubtitleConfig{URL: remote.URL, APIKey: "key"}, t.TempDir(), data, sidecarTestIndex(item), nil, "", testStateDatabase(t, data))
	key := subtitleSearchKey(item.ID, "en", provider.preference())

	if err := provider.fetchSidecar(t.Context(), item, "en"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("fetch into occupied sidecar = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("provider was queried %d times for an occupied sidecar", requests.Load())
	}
	record, found, err := provider.ledger.search(key)
	if err != nil || !found || record.Outcome != "exists" || record.SafeError == "" {
		t.Fatalf("search record = %#v, found=%t, err=%v", record, found, err)
	}
	if provider.ledger.automaticSearchReady(key, item, time.Now()) {
		t.Fatal("automation would retry the occupied sidecar on its next run")
	}
	if !provider.ledger.automaticSearchReady(key, item, time.Now().Add(25*time.Hour)) {
		t.Fatal("occupied sidecar is never checked again")
	}
}
