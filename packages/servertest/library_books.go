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

// ViewerCanBrowseAndDownloadEbooks preserves the Player book-reader regression contract.
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

// ReaderServesPDFEPUBAndComicPagesThroughAPIAndWeb preserves the Player book-reader regression contract.
func (fixture LibraryAPIFixture) ReaderServesPDFEPUBAndComicPagesThroughAPIAndWeb(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Manual.pdf"), []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	archivetest.WriteZIP(t, filepath.Join(media, "Novel.epub"), map[string]string{
		"META-INF/container.xml": `<?xml version="1.0"?><container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<package><manifest><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="one"/></spine></package>`,
		"OEBPS/one.xhtml":        `<html><body><h1>Chapter One</h1></body></html>`,
	})
	archivetest.WriteZIP(t, filepath.Join(media, "Comic.cbz"), map[string]string{"001.jpg": "one", "002.jpg": "two"})
	handler := fixture.NewHandler(media, "", false)
	libraryResponse := APICall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	for _, title := range []string{"Manual", "Novel", "Comic"} {
		id := regexp.MustCompile(`"id":"([a-f0-9]+)","kind":"book","title":"` + title + `"`).FindStringSubmatch(libraryResponse.Body.String())[1]
		manifest := APICall(t, handler, "", http.MethodGet, "/api/v1/books/"+id+"/reader", nil)
		AssertAPIBody(t, manifest, http.StatusOK, `"type":`, `/read/`+id)
		reader := APICall(t, handler, "", http.MethodGet, "/read/"+id, nil)
		AssertAPIBody(t, reader, http.StatusOK, title)
	}
}

// ReaderServesMainstreamComicArchivesThroughAPIAndWeb preserves the Player book-reader regression contract.
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
