package server_test

import (
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
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
	for name, body := range map[string]string{"malformed JSON": "{", "unknown field": `{"page":1,"unexpected":true}`, "missing page": `{}`} {
		if response := rawAPIRequest(t, handler, "", http.MethodPut, "/api/v1/books/"+id+"/reader/progress", body); response.Code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d %q", name, response.Code, response.Body.String())
		}
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/books/"+id+"/reader/progress", map[string]any{"page": 2}), http.StatusOK, `"page":2`, `"total":2`)
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader/progress", nil), http.StatusOK, `"page":2`, `"total":2`)
	for _, page := range []int{0, 3} {
		if response := apiCall(t, handler, "", http.MethodPut, "/api/v1/books/"+id+"/reader/progress", map[string]any{"page": page}); response.Code != http.StatusBadRequest {
			t.Fatalf("page %d accepted: %d %q", page, response.Code, response.Body.String())
		}
	}
	page := apiCall(t, handler, "", http.MethodGet, "/read/"+id, nil)
	assertAPIBody(t, page, http.StatusOK, `data-reader-page="2"`, `aria-current="page">Chapter 2`, `/read/`+id+`/asset/OEBPS/two.xhtml`)
	if response := webFormCall(t, handler, "", "/read/"+id+"/progress", map[string][]string{"page": {"1"}}); response.Code != http.StatusSeeOther {
		t.Fatalf("web page selection = %d %q", response.Code, response.Body.String())
	}
	page = apiCall(t, handler, "", http.MethodGet, "/read/"+id, nil)
	assertAPIBody(t, page, http.StatusOK, `data-reader-page="1"`, `aria-current="page">Chapter 1`, `/read/`+id+`/asset/OEBPS/one.xhtml`)
	if strings.Contains(page.Body.String(), `aria-current="page">Chapter 2`) {
		t.Fatal("reader kept the old chapter selected")
	}
	if !regexp.MustCompile(`<iframe[^>]+src="/read/` + id + `/asset/OEBPS/one\.xhtml"`).MatchString(page.Body.String()) {
		t.Fatalf("reader did not resume the selected chapter: %q", page.Body.String())
	}
}
