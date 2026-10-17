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

func TestSubtitleDashboardShowsTheLibraryScanUntilTheFirstScanPublishes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "Film (2020).mkv"), []byte("video"), 0o600); err != nil {
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
	settings := newSettingsStore(root, t.TempDir(), nil)
	manager := newSubtitleManager(index, settings, newSubtitleProvider(SubtitleConfig{}, t.TempDir(), t.TempDir(), index, settings, ""), nil)
	pending := httptest.NewRecorder()
	manager.dashboard(pending, ownerRequest("/?view=library"))
	body := pending.Body.String()
	if pending.Code != http.StatusServiceUnavailable || pending.Header().Get("Retry-After") != "5" || pending.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("pending dashboard = %d %v", pending.Code, pending.Header())
	}
	for _, want := range []string{`id="subtitle-content"`, `data-library-loading data-retry-after="5"`, `<h2 id="subtitle-list-title" tabindex="-1">Scanning your Library</h2>`, `role="status"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("pending dashboard lacks %q: %s", want, body)
		}
	}
	if strings.Contains(body, "Coverage is unavailable.") {
		t.Fatalf("pending dashboard rendered the failure state: %s", body)
	}
	germanRequest := ownerRequest("/?view=library")
	germanRequest.Header.Set("Accept-Language", "de")
	german := httptest.NewRecorder()
	manager.dashboard(german, germanRequest)
	for _, want := range []string{`<h2 id="subtitle-list-title" tabindex="-1">Ihre Bibliothek wird gescannt</h2>`, `<p role="status">Kinosail Subtitles liest Ihre Medienordner. Die Untertitelabdeckung erscheint hier, sobald der Scan abgeschlossen ist.</p>`} {
		if !strings.Contains(german.Body.String(), want) || strings.Contains(german.Body.String(), "Scanning your") {
			t.Fatalf("German pending dashboard lacks %q: %s", want, german.Body.String())
		}
	}
	api := httptest.NewRecorder()
	manager.statusAPI(api, ownerRequest("/api/v1/subtitle-library?view=library"))
	if api.Code != http.StatusServiceUnavailable || api.Header().Get("Retry-After") != "5" {
		t.Fatalf("pending subtitle library API = %d %v", api.Code, api.Header())
	}
	close(release)
	deadline := time.Now().Add(5 * time.Second)
	for {
		loaded := httptest.NewRecorder()
		manager.dashboard(loaded, ownerRequest("/?view=library"))
		if loaded.Code == http.StatusOK && strings.Contains(loaded.Body.String(), "Film") && !strings.Contains(loaded.Body.String(), "data-library-loading") && loaded.Header().Get("Retry-After") == "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("dashboard after first scan = %d %v", loaded.Code, loaded.Header())
		}
		time.Sleep(10 * time.Millisecond)
	}
}
