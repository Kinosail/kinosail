package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestVersionedAPIAndWebPersistStateOnlyInSQLite(t *testing.T) {
	media, data := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Movie.mp4"), []byte("movie"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	item := firstAPIItemID(t, handler, owner.Value)
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/items/"+item+"/progress", map[string]any{"seconds": 42}), http.StatusOK, `"seconds":42`)
	response := requestWithCookie(t, handler, http.MethodPost, "/playlists", "name=Favorites", owner)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("web playlist = %d %q", response.Code, response.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/playlists/Favorites/items/"+item, map[string]any{"included": true}), http.StatusOK, `"included":true`)
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/playlists/Favorites/items/"+item, map[string]any{"included": false}), http.StatusOK, `"included":false`)
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/playlists/Favorites/items/"+item, map[string]any{"included": true}), http.StatusOK, `"included":true`)
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Temporary", "ids": []string{item}}), http.StatusCreated, `"Temporary"`)
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodDelete, "/api/v1/playlists/Temporary", nil), http.StatusNoContent, "")
	for _, name := range database.Documents {
		if _, err := os.Lstat(filepath.Join(data, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy %s remains: %v", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(data, database.Filename)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("SQLite state = %v, %v", info, err)
	}
	handler = server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/history", nil), http.StatusOK, `"seconds":42`)
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/playlists", nil), http.StatusOK, `"Favorites"`)
}
