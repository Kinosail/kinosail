package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func TestHomeShowsTheLibraryScanUntilTheFirstScanPublishes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Arrival (2016).mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	index := &libraryIndex{Index: catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: root, Namespace: "Movies"}}, "", time.Hour, func(ctx context.Context) (func(), error) {
		select {
		case <-release:
			return func() {}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, catalog.Decoration{StartupWait: time.Millisecond})}
	handler := showHome(index, newProgressStore(""), newListStore(""), newSettingsStore("", "", "", nil), nil, false)
	pending := httptest.NewRecorder()
	handler(pending, ownerRequest("/"))
	body := pending.Body.String()
	if pending.Code != http.StatusServiceUnavailable || pending.Header().Get("Retry-After") != "5" || pending.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("pending home = %d %v", pending.Code, pending.Header())
	}
	for _, want := range []string{`data-library-loading data-retry-after="5"`, `<h2 id="library-loading-title">Scanning your Library</h2>`, `role="status" data-library-loading-status`, `class="library-shell last-light library-loading-shell"`, `href="/settings"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("pending home lacks %q: %s", want, body)
		}
	}
	if strings.Contains(body, "Your Server is ready.") || strings.Contains(body, "Could not complete the request") {
		t.Fatalf("pending home rendered an empty or failed Library: %s", body)
	}
	germanRequest := ownerRequest("/")
	germanRequest.Header.Set("Accept-Language", "de")
	german := httptest.NewRecorder()
	handler(german, germanRequest)
	for _, want := range []string{`<h2 id="library-loading-title">Ihre Bibliothek wird gescannt</h2>`, `data-library-loading-status>Kinosail liest Ihre Medienordner. Ihre Bibliothek erscheint hier, sobald der Scan abgeschlossen ist.</p>`} {
		if !strings.Contains(german.Body.String(), want) || strings.Contains(german.Body.String(), "Scanning your") {
			t.Fatalf("German pending home lacks %q: %s", want, german.Body.String())
		}
	}
	fragment := httptest.NewRecorder()
	fragmentRequest := ownerRequest("/?view=movies&offset=100")
	fragmentRequest.Header.Set("X-Kinosail-Library-Page", "1")
	handler(fragment, fragmentRequest)
	if fragment.Code != http.StatusServiceUnavailable || fragment.Header().Get("Retry-After") != "5" || strings.Contains(fragment.Body.String(), "data-library-loading") {
		t.Fatalf("pending fragment = %d %v %q", fragment.Code, fragment.Header(), fragment.Body.String())
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		loaded := httptest.NewRecorder()
		handler(loaded, ownerRequest("/?view=movies"))
		if loaded.Code == http.StatusOK && strings.Contains(loaded.Body.String(), "Arrival") && !strings.Contains(loaded.Body.String(), "data-library-loading") && loaded.Header().Get("Retry-After") == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("home after first scan = %d %v", loaded.Code, loaded.Header())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
