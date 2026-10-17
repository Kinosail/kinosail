package catalogapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestLibraryAsksClientsToRetryUntilTheFirstScanPublishes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Film (2020).mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	index := catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, func(ctx context.Context) (func(), error) {
		select {
		case <-release:
			return func() {}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, catalog.Decoration{StartupWait: time.Millisecond})
	var progressMutex sync.Mutex
	var listMutex sync.RWMutex
	progress, listed := map[string]catalog.PlaybackState{}, map[string]bool{}
	handler := Library(func(request *http.Request) (catalog.Result, error) {
		return index.BrowseLibrary(request.Context(), request.URL.Query(), "en", func() catalog.BrowseAccess {
			return catalog.NewBrowseAccess(&progressMutex, &listMutex, &progress, &listed, "owner", true, func(library.Item) bool { return true })
		})
	}, func(_ *http.Request, item library.Item) any { return map[string]string{"id": item.ID} })
	get := func() *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		handler(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library?view=movies", nil))
		return response
	}
	pending := get()
	if pending.Code != http.StatusServiceUnavailable || pending.Header().Get("Retry-After") != "5" || pending.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("pending library = %d Retry-After=%q Cache-Control=%q %s", pending.Code, pending.Header().Get("Retry-After"), pending.Header().Get("Cache-Control"), pending.Body.String())
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		loaded := get()
		var page struct{ Total int }
		if loaded.Code == http.StatusOK && json.Unmarshal(loaded.Body.Bytes(), &page) == nil && page.Total == 1 && loaded.Header().Get("Retry-After") == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("library after first scan = %d Retry-After=%q %s", loaded.Code, loaded.Header().Get("Retry-After"), loaded.Body.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
