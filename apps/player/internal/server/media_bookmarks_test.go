package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestAudioBookmarksPersistAndDelete(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"Song.mp3", "Book.m4b"} {
		t.Run(file, func(t *testing.T) { checkAudioBookmarks(t, file) })
	}
}

func checkAudioBookmarks(t *testing.T, file string) {
	t.Helper()
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, file), []byte("audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := server.Config{MediaDir: media, DataDir: data}
	handler, id := formatTestItem(t, config)
	endpoint := "/api/v1/items/" + id + "/bookmarks"
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, endpoint, nil), http.StatusOK, `"bookmarks":[]`)
	response := apiCall(t, handler, "", http.MethodPost, endpoint, map[string]any{"title": " Chorus ", "seconds": 12.5})
	assertAPIBody(t, response, http.StatusCreated, `"title":"Chorus"`, `"seconds":12.5`)
	var saved struct{ Bookmarks []struct{ ID string } }
	mustJSON(t, response, &saved)
	if len(saved.Bookmarks) != 1 || len(saved.Bookmarks[0].ID) != 64 {
		t.Fatalf("missing Server-generated bookmark: %s", response.Body.String())
	}
	assertInvalidAudioBookmarks(t, handler, endpoint, response.Body.String())
	reopened := server.New(config)
	assertAPIBody(t, apiCall(t, reopened, "", http.MethodGet, endpoint, nil), http.StatusOK, saved.Bookmarks[0].ID, `"seconds":12.5`)
	assertAPIBody(t, apiCall(t, reopened, "", http.MethodDelete, endpoint+"/"+saved.Bookmarks[0].ID, nil), http.StatusOK, `"bookmarks":[]`)
}

func assertInvalidAudioBookmarks(t *testing.T, handler http.Handler, endpoint, before string) {
	t.Helper()
	for _, body := range []string{
		`{}`, `{`, `{"title":"Spot"}`, `{"title":"Spot","seconds":null}`,
		`{"title":"Spot","seconds":-1}`, `{"title":"Spot","seconds":31536001}`,
		`{"title":"Spot","seconds":1,"page":1}`, `{"title":"Spot","page":1}`,
		`{"title":"Spot","seconds":1,"offset":0.2}`, `{"title":"Spot","seconds":1,"id":"client"}`,
		`{"title":"Spot","seconds":1,"seconds":2}`, `{"title":"` + strings.Repeat("x", 1025) + `","seconds":1}`,
	} {
		assertAPIBody(t, rawAPIRequest(t, handler, "", http.MethodPost, endpoint, body), http.StatusBadRequest)
		if got := apiCall(t, handler, "", http.MethodGet, endpoint, nil).Body.String(); got != before {
			t.Fatalf("rejected bookmark changed state for %s", body)
		}
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodDelete, endpoint+"/invalid", nil), http.StatusBadRequest)
	if got := apiCall(t, handler, "", http.MethodGet, endpoint, nil).Body.String(); got != before {
		t.Fatal("invalid deletion changed bookmarks")
	}
}

func TestPhotoBookmarksRejectWithoutSaving(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Photo.jpg"), []byte("photo"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, id := formatTestItem(t, server.Config{MediaDir: media, DataDir: data})
	endpoint := "/api/v1/items/" + id + "/bookmarks"
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		assertAPIBody(t, apiCall(t, handler, "", method, endpoint, map[string]any{"title": "Spot", "seconds": 1}), http.StatusBadRequest, "does not support bookmarks")
	}
}
