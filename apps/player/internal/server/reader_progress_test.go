package server_test

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestEPUBReaderPersistsChapterProgressThroughAPIAndWeb(t *testing.T) {
	t.Parallel()
	media, data := t.TempDir(), t.TempDir()
	writeZip(t, filepath.Join(media, "Novel.epub"), map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<package><manifest><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/><item id="two" href="two.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="one"/><itemref idref="two"/></spine></package>`,
		"OEBPS/one.xhtml":        `<html><body><h1>One</h1></body></html>`,
		"OEBPS/two.xhtml":        `<html><body><h1>Two</h1></body></html>`,
	})
	handler := server.New(server.Config{MediaDir: media, DataDir: data})
	items := apiItemsByTitle(t, handler)
	id := items["Novel"]

	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader/progress", nil), http.StatusOK, `"page":1`, `"total":2`)
	assertInvalidReaderProgressFields(t, handler, id)
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/books/"+id+"/reader/progress", map[string]any{"page": 2}), http.StatusOK, `"page":2`, `"total":2`)
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader/progress", nil), http.StatusOK, `"page":2`, `"total":2`)
	for _, page := range []int{0, 3} {
		if response := apiCall(t, handler, "", http.MethodPut, "/api/v1/books/"+id+"/reader/progress", map[string]any{"page": page}); response.Code != http.StatusBadRequest {
			t.Fatalf("page %d accepted: %d %q", page, response.Code, response.Body.String())
		}
	}
	endpoint := "/api/v1/books/" + id + "/reader/progress?includeOffset=true"
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, endpoint, map[string]any{"page": 2, "offset": 0.625}), http.StatusOK, `"offset":0.625`)
	for _, body := range []string{`{"page":2,"offset":null}`, `{"page":2,"offset":"0.5"}`, `{"page":2,"offset":-0.1}`, `{"page":2,"offset":1.1}`, `{"page":2,"offset":1e999}`, `{"page":2,"offset":0.2,"offset":0.9}`, `{"page":1,"Page":2}`, `{"page":2,"offset":0.2,"extra":1}`} {
		if response := rawAPIRequest(t, handler, "", http.MethodPut, endpoint, body); response.Code != http.StatusBadRequest {
			t.Fatalf("accepted %s: %d", body, response.Code)
		}
		assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, endpoint, nil), http.StatusOK, `"page":2`, `"offset":0.625`)
	}
	bookmarkURL := "/api/v1/items/" + id + "/bookmarks?includeOffset=true"
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPost, bookmarkURL, map[string]any{"title": "My spot", "page": 2, "offset": 0.625}), http.StatusCreated, `"offset":0.625`)
	savedBookmarks := apiCall(t, handler, "", http.MethodGet, bookmarkURL, nil).Body.String()
	assertInvalidReaderBookmarks(t, handler, bookmarkURL, savedBookmarks)
	assertReaderOffsetOptIn(t, handler, endpoint, bookmarkURL)
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, endpoint, nil), http.StatusOK, `"offset":0.625`)
	if got := apiCall(t, handler, "", http.MethodGet, bookmarkURL, nil).Body.String(); got != savedBookmarks {
		t.Fatal("compatibility reads or invalid queries changed bookmarks")
	}
	reopened := server.New(server.Config{MediaDir: media, DataDir: data})
	assertAPIBody(t, apiCall(t, reopened, "", http.MethodGet, endpoint, nil), http.StatusOK, `"page":2`, `"offset":0.625`)
	assertAPIBody(t, apiCall(t, reopened, "", http.MethodGet, bookmarkURL, nil), http.StatusOK, `"offset":0.625`)
	page := apiCall(t, handler, "", http.MethodGet, "/read/"+id, nil)
	assertAPIBody(t, page, http.StatusOK, `data-reader-page="2"`, `aria-current="page">Chapter 2`, `/read/`+id+`/asset/OEBPS/two.xhtml`)
	assertAPIBody(t, webFormCall(t, handler, "", "/read/"+id+"/progress", map[string][]string{"page": {"1"}}), http.StatusSeeOther)
	page = apiCall(t, handler, "", http.MethodGet, "/read/"+id, nil)
	assertAPIBody(t, page, http.StatusOK, `data-reader-page="1"`, `aria-current="page">Chapter 1`, `/read/`+id+`/asset/OEBPS/one.xhtml`)
	if strings.Contains(page.Body.String(), `aria-current="page">Chapter 2`) {
		t.Fatal("reader kept the old chapter selected")
	}
	if !regexp.MustCompile(`<iframe[^>]+src="/read/` + id + `/asset/OEBPS/one\.xhtml"`).MatchString(page.Body.String()) {
		t.Fatalf("reader did not resume the selected chapter: %q", page.Body.String())
	}
}

func assertReaderOffsetOptIn(t *testing.T, handler http.Handler, endpoint, bookmarkURL string) {
	t.Helper()
	for _, endpointWithOffset := range []string{endpoint, bookmarkURL} {
		oldURL := strings.Split(endpointWithOffset, "?")[0]
		oldBody := apiCall(t, handler, "", http.MethodGet, oldURL, nil).Body.String()
		if strings.Contains(oldBody, `"offset"`) {
			t.Fatal("older clients received an unknown offset field")
		}
		for _, query := range []string{"includeOffset=", "includeOffset=false", "includeOffset=true&includeOffset=true", "includeOffset=%zz", "includeOffset=" + strings.Repeat("x", 2049)} {
			method := http.MethodPut
			body := `{"page":1,"offset":0.1}`
			if oldURL == strings.Split(bookmarkURL, "?")[0] {
				method = http.MethodPost
				body = `{"title":"New","page":1,"offset":0.1}`
			}
			if response := rawAPIRequest(t, handler, "", method, oldURL+"?"+query, body); response.Code != http.StatusBadRequest {
				t.Fatalf("accepted query %q: %d", query, response.Code)
			}
		}
	}
}

func assertInvalidReaderBookmarks(t *testing.T, handler http.Handler, bookmarkURL, savedBookmarks string) {
	t.Helper()
	for _, body := range []string{`{"title":"Spot","page":2,"offset":null}`, `{"title":"Spot","page":2,"offset":"0.5"}`, `{"title":"Spot","page":2,"offset":-0.1}`, `{"title":"Spot","page":2,"offset":1.1}`, `{"title":"Spot","seconds":2,"offset":0.5}`} {
		if response := rawAPIRequest(t, handler, "", http.MethodPost, bookmarkURL, body); response.Code != http.StatusBadRequest {
			t.Fatalf("accepted bookmark %s: %d", body, response.Code)
		}
		if got := apiCall(t, handler, "", http.MethodGet, bookmarkURL, nil).Body.String(); got != savedBookmarks {
			t.Fatal("invalid bookmark modified saved marks")
		}
	}
}

func assertInvalidReaderProgressFields(t *testing.T, handler http.Handler, id string) {
	t.Helper()
	for name, body := range map[string]string{"malformed JSON": "{", "unknown field": `{"page":1,"unexpected":true}`, "missing page": `{}`} {
		if response := rawAPIRequest(t, handler, "", http.MethodPut, "/api/v1/books/"+id+"/reader/progress", body); response.Code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d %q", name, response.Code, response.Body.String())
		}
	}
}
