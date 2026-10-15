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
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/server", map[string]any{"name": "Cabin"}), http.StatusOK)
	response := requestWithCookie(t, handler, http.MethodPost, "/settings/scans", "frequency=off", owner)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("web scan setting = %d %q", response.Code, response.Body.String())
	}
	for _, name := range database.Documents {
		if _, err := os.Lstat(filepath.Join(data, name)); !os.IsNotExist(err) {
			t.Fatalf("legacy %s remains: %v", name, err)
		}
	}
	if info, err := os.Stat(filepath.Join(data, database.Filename)); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("SQLite state = %v, %v", info, err)
	}
	handler = server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"name":"Cabin"`, `"scanFrequency":"off"`)
}
