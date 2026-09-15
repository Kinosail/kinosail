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

func TestPlaylistsSupportPersistentOrderAndDynamicRules(t *testing.T) { //nolint:cyclop,funlen // One workflow test covers the persistent playlist contract.
	t.Parallel()

	media, data := t.TempDir(), t.TempDir()
	for _, name := range []string{"Arrival.mp4", "Zodiac.mp4", "Ambient.mp3", "Audiobooks/Ancillary.m4b"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(media, name)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(media, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	libraryResponse := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct {
			ID, Title string
		} `json:"items"`
	}
	mustJSON(t, libraryResponse, &catalog)
	ids := map[string]string{}
	for _, item := range catalog.Items {
		ids[item.Title] = item.ID
	}
	assertAPICalls(t, handler, session.Token, []apiTestCall{
		{Method: http.MethodPost, Path: "/api/v1/playlists", Body: map[string]any{"name": "Films"}, Status: http.StatusCreated},
		{Method: http.MethodPut, Path: "/api/v1/playlists/Films/items/" + ids["Arrival"], Body: map[string]any{"included": true}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/playlists/Films/items/" + ids["Zodiac"], Body: map[string]any{"included": true}, Status: http.StatusOK},
		{Method: http.MethodPut, Path: "/api/v1/playlists/Films/order", Body: map[string]any{"ids": []string{ids["Zodiac"], ids["Arrival"]}}, Status: http.StatusOK},
		{Method: http.MethodPost, Path: "/api/v1/smart-playlists", Body: map[string]any{"name": "A titles", "kind": "video", "query": "a", "sort": "title"}, Status: http.StatusCreated},
	})

	handler = server.New(server.Config{MediaDir: media, DataDir: data, RequireAuth: true})
	manual := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/playlists/Films", nil)
	var playlist struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.Unmarshal(manual.Body.Bytes(), &playlist); err != nil || len(playlist.Items) != 2 || playlist.Items[0].Title != "Zodiac" {
		t.Fatalf("manual playlist = %d %q: %v", manual.Code, manual.Body.String(), err)
	}
	smart := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/playlists/A%20titles", nil)
	assertAPIBody(t, smart, http.StatusOK, "Arrival")
	if smart.Body.String() == "" || !json.Valid(smart.Body.Bytes()) {
		t.Fatalf("smart playlist JSON = %q", smart.Body.String())
	}
	player := apiCall(t, handler, session.Token, http.MethodGet, "/watch/"+ids["Arrival"], nil)
	assertAPIBody(t, player, http.StatusOK, `aria-label="Remove from playlist · Films"`, `<span>Films</span><small>Added</small>`)
	if strings.Contains(player.Body.String(), "A titles") {
		t.Fatalf("smart playlist exposed a manual membership control: %q", player.Body.String())
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/smart-playlists", strings.NewReader("name=Music&kind=audio&sort=title"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Authorization", "Bearer "+session.Token)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("smart playlist web adapter = %d %q", response.Code, response.Body.String())
	}
	audio := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/playlists/Music", nil)
	assertAPIBody(t, audio, http.StatusOK, "Ambient", "Ancillary")
}

func TestPlaylistDocumentsRoundTripThroughVersionedAPI(t *testing.T) { //nolint:cyclop // One scenario covers validation, persistence, and export.
	t.Parallel()

	handler, token := apiServer(t)
	library := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct{ ID string } `json:"items"`
	}
	mustJSON(t, library, &catalog)
	if len(catalog.Items) < 2 {
		t.Fatalf("test Library has %d items, want at least 2", len(catalog.Items))
	}
	ids := []string{catalog.Items[1].ID, catalog.Items[0].ID}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/playlists", map[string]any{"name": "Source", "ids": ids}), http.StatusCreated)

	exported := apiCall(t, handler, token, http.MethodGet, "/api/v1/playlists/Source?format=kinosail", nil)
	if exported.Code != http.StatusOK || exported.Header().Get("Content-Type") != "application/json" || !strings.Contains(exported.Header().Get("Content-Disposition"), "kinosail-playlist.json") {
		t.Fatalf("export = %d, type=%q disposition=%q body=%q", exported.Code, exported.Header().Get("Content-Type"), exported.Header().Get("Content-Disposition"), exported.Body.String())
	}
	var document struct {
		Format  string   `json:"format"`
		Version int      `json:"version"`
		Name    string   `json:"name"`
		IDs     []string `json:"ids"`
	}
	mustJSON(t, exported, &document)
	if document.Format != "kinosail.playlist" || document.Version != 1 || document.Name != "Source" || !equalStrings(document.IDs, ids) {
		t.Fatalf("export document = %#v, want versioned ordered playlist", document)
	}
	document.Name = "Imported"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/playlists", document), http.StatusCreated, `"name":"Imported"`)
	imported := apiCall(t, handler, token, http.MethodGet, "/api/v1/playlists/Imported", nil)
	if imported.Code != http.StatusOK || !strings.Contains(imported.Body.String(), ids[0]) || strings.Index(imported.Body.String(), ids[0]) > strings.Index(imported.Body.String(), ids[1]) {
		t.Fatalf("imported playlist = %d %q", imported.Code, imported.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodGet, "/api/v1/playlists/Source?format=other", nil), http.StatusBadRequest, "playlist format is invalid")

	document.Name, document.Format = "Rejected", "other.playlist"
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/playlists", document), http.StatusBadRequest, "playlist document is invalid")
	document.Format, document.IDs = "kinosail.playlist", []string{"missing"}
	assertAPIBody(t, apiCall(t, handler, token, http.MethodPost, "/api/v1/playlists", document), http.StatusBadRequest, "playlist item is unavailable")
	playlists := apiCall(t, handler, token, http.MethodGet, "/api/v1/playlists", nil)
	if strings.Contains(playlists.Body.String(), `"name":"Rejected"`) {
		t.Fatalf("rejected import changed playlist state: %q", playlists.Body.String())
	}
}

func TestPlaylistDocumentsAreReachableThroughWebAdapter(t *testing.T) {
	t.Parallel()

	handler, token := apiServer(t)
	library := apiCall(t, handler, token, http.MethodGet, "/api/v1/library", nil)
	var catalog struct {
		Items []struct{ ID string } `json:"items"`
	}
	mustJSON(t, library, &catalog)
	document, err := json.Marshal(map[string]any{"format": "kinosail.playlist", "version": 1, "name": "Web import", "ids": []string{catalog.Items[0].ID}})
	if err != nil {
		t.Fatal(err)
	}
	created := webFormCall(t, handler, token, "/playlists", url.Values{"document": {string(document)}})
	if created.Code != http.StatusSeeOther || !strings.Contains(created.Header().Get("Location"), "Web%20import") {
		t.Fatalf("web import = %d location=%q body=%q", created.Code, created.Header().Get("Location"), created.Body.String())
	}
	playlists := apiCall(t, handler, token, http.MethodGet, "/?view=playlists", nil)
	assertAPIBody(t, playlists, http.StatusOK, "Import playlist", `name="document"`)
	detail := apiCall(t, handler, token, http.MethodGet, "/playlist/Web%20import", nil)
	assertAPIBody(t, detail, http.StatusOK, "Export playlist", "format=kinosail")

	invalid := webFormCall(t, handler, token, "/playlists", url.Values{"document": {`{"format":"kinosail.playlist","version":1,"name":"No side effect","ids":["missing"]}`}})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid web import = %d %q", invalid.Code, invalid.Body.String())
	}
	playlists = apiCall(t, handler, token, http.MethodGet, "/api/v1/playlists", nil)
	if strings.Contains(playlists.Body.String(), `"name":"No side effect"`) {
		t.Fatalf("invalid web import changed playlist state: %q", playlists.Body.String())
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
