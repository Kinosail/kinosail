package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestViewerCanCreateAndUsePersistentPlaylist(t *testing.T) {
	t.Parallel()

	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Arrival.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	postForm(t, handler, "/playlists", "name=Favorites")
	browser := httptest.NewRecorder()
	handler.ServeHTTP(browser, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=playlists", nil))
	search := httptest.NewRecorder()
	handler.ServeHTTP(search, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playlist/Favorites?q=Arrival", nil))
	assertInvalidPlaylistForms(t, handler, id)
	added := collectionForm(t, handler, "/playlist/Favorites/items/"+id, "included=true&q=Arrival")

	handler = server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir})
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playlist/Favorites", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	if added.Header().Get("Location") != "/playlist/Favorites?q=Arrival#playlist-search" || playlist.Code != http.StatusOK || !strings.Contains(playlist.Body.String(), "Arrival") || !strings.Contains(playlist.Body.String(), "Delete Favorites?") || !strings.Contains(playlist.Body.String(), "Delete playlist") || !strings.Contains(player.Body.String(), "Remove from") || !strings.Contains(player.Body.String(), "Favorites") {
		t.Fatalf("playlist = %d %q, player = %q", playlist.Code, playlist.Body.String(), player.Body.String())
	}
	mustContainAll(t, browser.Body.String(), `aria-current="page" href="/?view=playlists">Playlists`, `class="curation-card"`, "Manual · 0 items")
	mustContainAll(t, search.Body.String(), "/playlist/Favorites/items/"+id, "Add to")
	removed := collectionForm(t, handler, "/playlist/Favorites/items/"+id, "included=false&_csrf=transport-token")
	if removed.Code != http.StatusSeeOther {
		t.Fatalf("remove = %d %q", removed.Code, removed.Body.String())
	}
	empty := httptest.NewRecorder()
	handler.ServeHTTP(empty, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playlist/Favorites", nil))
	mustContainAll(t, empty.Body.String(), "This playlist is empty.")
}

func TestViewerCanReorderPlaylistWithoutDragging(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	for _, title := range []string{"Alpha", "Beta", "Gamma"} {
		if err := os.WriteFile(filepath.Join(media, title+".mp4"), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := server.New(server.Config{MediaDir: media, DataDir: data})
	postForm(t, handler, "/playlists", "name=Queue")
	library := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies", nil)
	var catalog struct {
		Items []struct{ ID, Title string } `json:"items"`
	}
	if err := json.Unmarshal(library.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	for _, item := range catalog.Items {
		collectionForm(t, handler, "/playlist/Queue/items/"+item.ID, "included=true")
	}
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playlist/Queue", nil))
	mustContainAll(t, page.Body.String(), `aria-label="Move Alpha later"`, `aria-label="Move Beta earlier"`, `class="playlist-order"`)

	moved := collectionForm(t, handler, "/playlist/Queue/order", "id="+catalog.Items[1].ID+"&direction=up")
	if moved.Code != http.StatusSeeOther {
		t.Fatalf("move = %d %q", moved.Code, moved.Body.String())
	}
	ordered := apiCall(t, handler, "", http.MethodGet, "/api/v1/playlists/Queue", nil)
	if strings.Index(ordered.Body.String(), `"title":"Beta"`) > strings.Index(ordered.Body.String(), `"title":"Alpha"`) {
		t.Fatalf("playlist order = %q", ordered.Body.String())
	}
	for _, body := range []string{"", "id=" + catalog.Items[1].ID, "id=" + catalog.Items[1].ID + "&direction=sideways", "id=" + catalog.Items[1].ID + "&direction=up&direction=down", "id=" + catalog.Items[1].ID + "&direction=up&unknown=x", "id=" + strings.Repeat("a", 65) + "&direction=up"} {
		invalid := collectionForm(t, handler, "/playlist/Queue/order", body)
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("invalid move %q = %d %q", body, invalid.Code, invalid.Body.String())
		}
	}
	unchanged := apiCall(t, handler, "", http.MethodGet, "/api/v1/playlists/Queue", nil)
	if strings.Index(unchanged.Body.String(), `"title":"Beta"`) > strings.Index(unchanged.Body.String(), `"title":"Alpha"`) {
		t.Fatalf("invalid moves changed playlist = %q", unchanged.Body.String())
	}
}

func assertInvalidPlaylistForms(t *testing.T, handler http.Handler, id string) {
	t.Helper()
	path := "/playlist/Favorites/items/" + id
	for _, input := range []struct{ Path, Body string }{
		{path, ""},
		{path, "included=maybe"},
		{path, "included=true&included=false"},
		{path, "included=true&unknown=x"},
		{path, "included=true&q=" + strings.Repeat("x", 201)},
		{path + "?unknown=x", "included=true"},
		{"/playlist/Favorites/items/not-an-id", "included=true"},
		{"/playlist/" + strings.Repeat("x", 65) + "/items/" + id, "included=true"},
	} {
		response := collectionForm(t, handler, input.Path, input.Body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid playlist form %s %q = %d %q", input.Path, input.Body, response.Code, response.Body.String())
		}
	}
	unchanged := httptest.NewRecorder()
	handler.ServeHTTP(unchanged, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/playlist/Favorites", nil))
	if !strings.Contains(unchanged.Body.String(), "0 items") {
		t.Fatalf("invalid membership forms changed playlist: %q", unchanged.Body.String())
	}
}

func postForm(t *testing.T, handler http.Handler, path, body string) {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusSeeOther {
		t.Fatalf("POST %s = %d %q", path, response.Code, response.Body.String())
	}
}
