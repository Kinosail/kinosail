package servertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func (fixture LibraryAPIFixture) LibraryPaginationIsSharedByAPIAndWeb(t *testing.T) {
	handler := fixture.PaginatedLibrary(t)
	api := APICall(t, handler, "", http.MethodGet, "/api/v1/library?view=movies&limit=1&offset=1", nil)
	assertLibraryAPIPage(t, api)
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&limit=1&offset=1", nil))
	if web.Code != http.StatusOK || len(regexp.MustCompile(`href="/(?:item|watch)/[a-f0-9]+"`).FindAllString(web.Body.String(), -1)) != 1 || !strings.Contains(web.Body.String(), `offset=0`) || !strings.Contains(web.Body.String(), `offset=2`) || !strings.Contains(web.Body.String(), `data-library-group="movies"`) || !strings.Contains(web.Body.String(), `data-library-next`) {
		t.Fatalf("web page = %d %q", web.Code, web.Body.String())
	}
}

func assertLibraryAPIPage(t *testing.T, api *httptest.ResponseRecorder) {
	t.Helper()
	var page struct {
		Items  []any `json:"items"`
		Total  int   `json:"total"`
		Offset int   `json:"offset"`
		Limit  int   `json:"limit"`
	}
	if err := json.Unmarshal(api.Body.Bytes(), &page); err != nil || api.Code != http.StatusOK || len(page.Items) != 1 || page.Total != 3 || page.Offset != 1 || page.Limit != 1 {
		t.Fatalf("API page = %#v, status = %d, error = %v", page, api.Code, err)
	}
}

func (fixture LibraryAPIFixture) DeepLibraryPageDoesNotRepeatHomeShelves(t *testing.T) {
	handler := fixture.PaginatedLibrary(t)
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?limit=1&offset=1", nil))
	if web.Code != http.StatusOK || strings.Contains(web.Body.String(), "Recently added") || strings.Contains(web.Body.String(), "Recently played") {
		t.Fatalf("deep Library page = %d %q", web.Code, web.Body.String())
	}
}

func (fixture LibraryAPIFixture) InfiniteLibraryPageReturnsOnlyTheBoundedFragment(t *testing.T) {
	handler := fixture.PaginatedLibrary(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&limit=1&offset=1", nil)
	request.Header.Set("X-Kinosail-Library-Page", "1")
	fragment := httptest.NewRecorder()
	handler.ServeHTTP(fragment, request)
	if fragment.Code != http.StatusOK || len(regexp.MustCompile(`href="/(?:item|watch)/[a-f0-9]+"`).FindAllString(fragment.Body.String(), -1)) != 1 || !strings.Contains(fragment.Body.String(), `data-library-group="movies"`) || !strings.Contains(fragment.Body.String(), `data-library-next`) || strings.Contains(fragment.Body.String(), "<!doctype html>") || strings.Contains(fragment.Body.String(), "Library organization") {
		t.Fatalf("infinite Library fragment = %d %q", fragment.Code, fragment.Body.String())
	}
	for _, value := range []string{"true", "1, 1", strings.Repeat("1", 1024)} {
		request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&limit=1&offset=1", nil)
		request.Header.Set("X-Kinosail-Library-Page", value)
		rejected := httptest.NewRecorder()
		handler.ServeHTTP(rejected, request)
		if rejected.Code != http.StatusBadRequest {
			t.Fatalf("infinite Library header %q = %d %q", value, rejected.Code, rejected.Body.String())
		}
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=movies&limit=1&offset=1", nil)
	request.Header.Add("X-Kinosail-Library-Page", "1")
	request.Header.Add("X-Kinosail-Library-Page", "1")
	rejected := httptest.NewRecorder()
	handler.ServeHTTP(rejected, request)
	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("duplicate infinite Library header = %d %q", rejected.Code, rejected.Body.String())
	}
}

func (fixture LibraryAPIFixture) PaginatedLibrary(t *testing.T) http.Handler {
	t.Helper()
	mediaDir := t.TempDir()
	for _, name := range []string{"Alpha.mp4", "Beta.mp4", "Gamma.mp4"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return fixture.NewHandler(mediaDir, "", false)
}

func (fixture LibraryAPIFixture) LibraryPaginationRejectsAmbiguousAndOutOfRangeInput(t *testing.T) {
	handler := fixture.NewHandler("", "", false)
	for _, path := range []string{"/api/v1/library?limit=0", "/api/v1/library?limit=201", "/api/v1/library?offset=-1", "/api/v1/library?limit=1&limit=2", "/api/v1/library?view=unknown", "/api/v1/library?sort=unknown", "/api/v1/library?unexpected=true", "/api/v1/library?letter=", "/api/v1/library?letter=%20", "/api/v1/library?letter=%CC%81", "/api/v1/library?letter=A1", "/api/v1/library?letter=G&letter=H", "/api/v1/library?letter=TOOLONGGG", "/api/v1/library?letter=%21", "/api/v1/library?letter=G&offset=1", "/api/v1/library?letter=G&sort=year", "/api/v1/library?letter=G"} {
		response := APICall(t, handler, "", http.MethodGet, path, nil)
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"error"`) {
			t.Fatalf("%s = %d %q", path, response.Code, response.Body.String())
		}
	}
	web := httptest.NewRecorder()
	handler.ServeHTTP(web, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?limit=invalid", nil))
	if web.Code != http.StatusBadRequest {
		t.Fatalf("web invalid pagination = %d %q", web.Code, web.Body.String())
	}
}
