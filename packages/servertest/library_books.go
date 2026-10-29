package servertest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/archivetest"
)

func (fixture LibraryAPIFixture) ViewerCanBrowseAndDownloadEbooks(t *testing.T) {
	mediaDir := t.TempDir()
	for name, content := range map[string]string{"Dune.epub": "epub", "Manual.pdf": "%PDF"} {
		if err := os.WriteFile(filepath.Join(mediaDir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	handler := fixture.NewHandler(mediaDir, "", false)
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/?view=books", nil))
	match := regexp.MustCompile(`/book/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if len(match) != 2 {
		t.Fatalf("home has no Book link: %q", home.Body.String())
	}
	for _, expected := range []string{"Books", "Dune", "Manual"} {
		if !strings.Contains(home.Body.String(), expected) {
			t.Fatalf("home lacks %q: %q", expected, home.Body.String())
		}
	}
	book := httptest.NewRecorder()
	handler.ServeHTTP(book, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/book/"+match[1], nil))
	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/download/"+match[1], nil))
	if book.Code != http.StatusOK || !strings.Contains(book.Body.String(), `/download/`+match[1]) || download.Code != http.StatusOK || download.Header().Get("Content-Disposition") == "" {
		t.Fatalf("book = %d %q, download = %d %q", book.Code, book.Body.String(), download.Code, download.Header())
	}
}

func (fixture LibraryAPIFixture) ReaderServesMainstreamComicArchivesThroughAPIAndWeb(t *testing.T, itemsByTitle func(*testing.T, http.Handler) map[string]string) {
	if _, err := exec.LookPath("bsdtar"); err != nil {
		t.Fatalf("bsdtar runtime is required for CB7 and CBT reader coverage: %v", err)
	}
	media := t.TempDir()
	for _, extension := range []string{"cb7", "cbt"} {
		archivetest.WriteTar(t, filepath.Join(media, "Comic "+strings.ToUpper(extension)+"."+extension), map[string]string{"-page.jpg": "safe option name", "001.jpg": "one", "002.webp": "two", "notes.txt": "ignored"})
	}
	handler := fixture.NewHandler(media, "", false)
	items := itemsByTitle(t, handler)
	for title, id := range items {
		manifest := APICall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
		AssertAPIBody(t, manifest, http.StatusOK, `"type":"comic"`, "001.jpg", "002.webp")
		reader := APICall(t, handler, "", http.MethodGet, "/read/"+id, nil)
		AssertAPIBody(t, reader, http.StatusOK, title, `class="reader-pages"`)
		asset := APICall(t, handler, "", http.MethodGet, "/read/"+id+"/asset/-page.jpg", nil)
		AssertAPIBody(t, asset, http.StatusOK, "safe option name")
	}
}
