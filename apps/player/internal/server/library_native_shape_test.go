package server_test

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestLibraryListKeepsTheNativeClientShapeUnlessFacetsAreRequested(t *testing.T) {
	t.Parallel()
	handler := libraryFilterHandler(t)
	native := []string{"items", "view", "sort", "query", "letter", "total", "offset", "limit", "letters"}
	keys := func(query string) []string {
		t.Helper()
		response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?"+query, nil)
		if response.Code != http.StatusOK {
			t.Fatalf("GET library?%s = %d %q", query, response.Code, response.Body.String())
		}
		var page map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		var extra []string
		for key := range page {
			if !slices.Contains(native, key) {
				extra = append(extra, key)
			}
		}
		slices.Sort(extra)
		return extra
	}
	for _, query := range []string{"", "view=movies", "q=Arrival", "view=movies&genre=Drama", "view=movies&watched=unwatched"} {
		if extra := keys(query); len(extra) != 0 {
			t.Errorf("library?%s adds %v, which released iPhone, Apple TV and Android apps reject", query, extra)
		}
	}
	if extra := keys("view=movies&facets=true"); !slices.Equal(extra, []string{"facets", "filters"}) {
		t.Errorf("library?view=movies&facets=true adds %v, want facets and filters", extra)
	}
	for _, query := range []string{"facets=false", "facets=1", "facets=", "facets=true&facets=true", "facets=" + strings.Repeat("t", 9)} {
		response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&"+query, nil)
		if response.Code != http.StatusBadRequest || strings.Contains(response.Body.String(), `"items"`) {
			t.Errorf("library?%s = %d %q", query, response.Code, response.Body.String())
		}
	}
}
