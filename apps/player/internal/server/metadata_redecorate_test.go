package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func libraryTitles(t *testing.T, handler http.Handler) map[string]string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/library", nil))
	var page struct{ Items []struct{ ID, Title string } }
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &page) != nil {
		t.Fatalf("library = %d %q", response.Code, response.Body.String())
	}
	titles := make(map[string]string, len(page.Items))
	for _, item := range page.Items {
		titles[item.ID] = item.Title
	}
	return titles
}

func TestMetadataEditsRepublishWithoutRescanningTheLibrary(t *testing.T) {
	mediaDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Primer (2004).mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: t.TempDir(), CacheDir: t.TempDir()})
	var id string
	for candidate := range libraryTitles(t, handler) {
		id = candidate
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Unscanned (2010).mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	form := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/metadata/"+id, strings.NewReader(url.Values{"title": {"Edited on the web"}}.Encode()))
	form.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	edited := httptest.NewRecorder()
	handler.ServeHTTP(edited, form)
	if titles := libraryTitles(t, handler); edited.Code != http.StatusSeeOther || len(titles) != 1 || titles[id] != "Edited on the web" {
		t.Fatalf("web edit = %d, library = %#v", edited.Code, titles)
	}
	api := httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/api/v1/items/"+id+"/metadata", strings.NewReader(`{"title":"Edited through the API"}`))
	api.Header.Set("Content-Type", "application/json")
	updated := httptest.NewRecorder()
	handler.ServeHTTP(updated, api)
	if titles := libraryTitles(t, handler); updated.Code != http.StatusOK || len(titles) != 1 || titles[id] != "Edited through the API" {
		t.Fatalf("API edit = %d %q, library = %#v", updated.Code, updated.Body.String(), titles)
	}
}
