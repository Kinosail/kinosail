package catalog

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCollectionReadAcceptsEncodedMetadataName(t *testing.T) {
	t.Parallel()
	name := "28 Days/Weeks/Years Later Collection"
	index := &collectionIndexStub{}
	store := &collectionStoreStub{names: []string{name}, selected: []library.Item{{ID: "sample"}}}
	progress := &collectionProgressStub{}
	handler := CollectionHandlers{Index: index, Lists: store, Progress: progress}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/collections/{name}", handler.Get)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/collections/"+url.PathEscape(name), nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), name) || progress.calls != 1 {
		t.Fatalf("collection = %d %s, projections = %d", response.Code, response.Body, progress.calls)
	}
	if store.creates+store.saves+store.deletes != 0 {
		t.Fatal("collection read mutated storage")
	}
}

func TestCollectionReadRejectsInvalidAndUnknownNamesWithoutProjection(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"", " ", ".", "..", "../private", "a/../b", "a/./b", "a\\b", " padded", "bad\nname", strings.Repeat("a", 65), "unknown"} {
		t.Run(name, func(t *testing.T) {
			projected := false
			action := CollectionAPI(name, func() []library.Item { return nil }, func([]library.Item) []string { return []string{"Visible"} },
				func(string, []library.Item) []library.Item { t.Fatal("unexpected collection lookup"); return nil },
				func([]library.Item) any { projected = true; return nil })
			if !action.NotFound || projected {
				t.Fatalf("invalid collection returned %#v", action)
			}
		})
	}
}
