package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestOwnerCanCreateAndCurateCollection(t *testing.T) { //nolint:cyclop,funlen // One lifecycle proves validation, persistence, and both adapters.
	t.Parallel()
	mediaDir, dataDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune.mp4"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mediaDir, "Dune Guide.epub"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir})
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	watches := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindAllStringSubmatch(home.Body.String(), -1)
	id := watches[len(watches)-1][1]
	bookID := regexp.MustCompile(`/book/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	create := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/collections", strings.NewReader("name=Favorites"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(create, request)
	browser := httptest.NewRecorder()
	handler.ServeHTTP(browser, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=collections", nil))
	api := httptest.NewRecorder()
	handler.ServeHTTP(api, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/collections", nil))
	search := httptest.NewRecorder()
	handler.ServeHTTP(search, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/collection/Favorites?q=Dune", nil))
	player := httptest.NewRecorder()
	handler.ServeHTTP(player, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id, nil))
	bookPage := httptest.NewRecorder()
	handler.ServeHTTP(bookPage, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/book/"+bookID, nil))

	assertInvalidCollectionForms(t, handler, id)
	unchanged := httptest.NewRecorder()
	handler.ServeHTTP(unchanged, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/collection/Favorites", nil))
	if !strings.Contains(unchanged.Body.String(), "0 items") {
		t.Fatalf("invalid membership forms changed collection: %q", unchanged.Body.String())
	}
	add := collectionForm(t, handler, "/collection/Favorites/items/"+id, "included=true&q=Dune")
	book := collectionForm(t, handler, "/collection/Favorites/items/"+bookID, "included=true")
	collection := httptest.NewRecorder()
	server.New(server.Config{MediaDir: mediaDir, DataDir: dataDir}).ServeHTTP(collection, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/collection/Favorites", nil))

	if create.Code != http.StatusSeeOther || add.Header().Get("Location") != "/collection/Favorites?q=Dune#collection-search" || book.Header().Get("Location") != "/collection/Favorites" || collection.Code != http.StatusOK {
		t.Fatalf("create = %d, add = %q, book = %q, collection = %d %q", create.Code, add.Header().Get("Location"), book.Header().Get("Location"), collection.Code, collection.Body.String())
	}
	mustContainAll(t, browser.Body.String(), `class="active" aria-current="page" href="/?view=collections">Collections`, "Custom collections", "Default collections", "No default collections found in your media metadata.", `class="curation-card"`, "0 items")
	mustContainAll(t, api.Body.String(), `"name":"Favorites"`, `"source":"custom"`)
	mustContainAll(t, search.Body.String(), "/collection/Favorites/items/"+id, "/collection/Favorites/items/"+bookID, "Add to")
	for name, page := range map[string]string{"player": player.Body.String(), "book": bookPage.Body.String()} {
		mustContainAll(t, page, `class="curation-options"`, `<h2>Collections</h2>`, `<span>Favorites</span><small>Add</small>`, `aria-label="Add to Collection · Favorites"`)
		if strings.Contains(page, `>Add to Collection · Favorites</button>`) {
			t.Fatalf("%s repeats the collection action in its visible label: %q", name, page)
		}
	}
	styles := httptest.NewRecorder()
	handler.ServeHTTP(styles, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/static/app.css", nil))
	mustContainAll(t, collection.Body.String(), `class="detail-shell collection-detail collection-page"`, "2 items", "Dune Guide", "Remove", "Delete Favorites?", "Delete Collection")
	mustContainAll(t, styles.Body.String(), ".collection-page .collection-items .card{display:grid", ".collection-page .collection-results .card{display:grid", ".collection-page .collection-items .card button,.collection-page .collection-results .card button{min-height:44px}")

	removed := collectionForm(t, handler, "/collection/Favorites/items/"+id, "included=false")
	after := httptest.NewRecorder()
	handler.ServeHTTP(after, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/collection/Favorites", nil))
	if removed.Code != http.StatusSeeOther || strings.Contains(after.Body.String(), `<h2>Dune</h2>`) || !strings.Contains(after.Body.String(), "1 items") {
		t.Fatalf("remove = %d, collection = %q", removed.Code, after.Body.String())
	}
}

func assertInvalidCollectionForms(t *testing.T, handler http.Handler, id string) {
	t.Helper()
	path := "/collection/Favorites/items/" + id
	inputs := []struct{ Path, Body string }{
		{path, ""},
		{path, "included=maybe"},
		{path, "included=true&included=false"},
		{path, "included=true&unknown=x"},
		{path, "included=true&q=" + strings.Repeat("x", 201)},
		{path + "?unknown=x", "included=true"},
		{"/collection/Favorites/items/not-an-id", "included=true"},
		{"/collection/" + strings.Repeat("x", 201) + "/items/" + id, "included=true"},
	}
	for _, input := range inputs {
		invalid := collectionForm(t, handler, input.Path, input.Body)
		if invalid.Code != http.StatusBadRequest {
			t.Fatalf("invalid collection form %s %q = %d %q", input.Path, input.Body, invalid.Code, invalid.Body.String())
		}
	}
}

func TestCollectionSearchRejectsAmbiguousInput(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{MediaDir: t.TempDir(), DataDir: t.TempDir()})
	create := collectionForm(t, handler, "/collections", "name=Favorites")
	if create.Code != http.StatusSeeOther {
		t.Fatalf("create = %d %q", create.Code, create.Body.String())
	}
	for _, path := range []string{"/collection/Favorites?unknown=x", "/collection/Favorites?q=a&q=b", "/collection/Favorites?q=" + strings.Repeat("x", 201)} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
}

func collectionForm(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(response, request)
	return response
}
