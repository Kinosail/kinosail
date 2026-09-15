package server_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestReaderRejectsMalformedAndEmptyBookArchives(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Broken.cbz"), []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Broken CB7.cb7", "Broken CBT.cbt"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte("not an archive"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeZip(t, filepath.Join(media, "Empty.cbz"), map[string]string{"notes.txt": "no pages"})
	writeZip(t, filepath.Join(media, "Traversal.cbz"), map[string]string{"../escape.jpg": "escape", "/absolute.jpg": "absolute"})
	writeZip(t, filepath.Join(media, "MissingContainer.epub"), map[string]string{"chapter.xhtml": "chapter"})
	writeZip(t, filepath.Join(media, "BadContainer.epub"), map[string]string{"META-INF/container.xml": "<invalid"})
	writeZip(t, filepath.Join(media, "EmptyContainer.epub"), map[string]string{"META-INF/container.xml": "<container></container>"})
	writeZip(t, filepath.Join(media, "MissingPackage.epub"), map[string]string{"META-INF/container.xml": `<container><rootfiles><rootfile full-path="missing.opf"/></rootfiles></container>`})
	writeZip(t, filepath.Join(media, "BadPackage.epub"), map[string]string{"META-INF/container.xml": `<container><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`, "content.opf": "<invalid"})
	writeZip(t, filepath.Join(media, "EmptySpine.epub"), map[string]string{"META-INF/container.xml": `<container><rootfiles><rootfile full-path="content.opf"/></rootfiles></container>`, "content.opf": `<package><manifest><item id="one" href="one.xhtml" media-type="image/jpeg"/></manifest><spine><itemref idref="missing"/></spine></package>`})

	handler := server.New(server.Config{MediaDir: media})
	items := apiItemsByTitle(t, handler)
	for _, title := range []string{"Broken", "Broken CB7", "Broken CBT", "Empty", "Traversal", "MissingContainer", "BadContainer", "EmptyContainer", "MissingPackage", "BadPackage", "EmptySpine"} {
		response := apiCall(t, handler, "", http.MethodGet, "/api/v1/books/"+items[title]+"/reader", nil)
		if response.Code != http.StatusNotFound {
			t.Fatalf("reader for %s = %d %q", title, response.Code, response.Body.String())
		}
	}
}

func TestReaderServesOnlyValidatedBookFilesAndAssets(t *testing.T) {
	media := t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Manual.pdf"), []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	writeZip(t, filepath.Join(media, "Novel.epub"), map[string]string{
		"META-INF/container.xml": `<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>`,
		"OEBPS/content.opf":      `<package><manifest><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/></manifest><spine><itemref idref="one"/></spine></package>`,
		"OEBPS/one.xhtml":        `<html><body>Chapter</body></html>`,
	})
	handler := server.New(server.Config{MediaDir: media})
	items := apiItemsByTitle(t, handler)
	for path, status := range map[string]int{
		"/read/" + items["Manual"] + "/file":                     http.StatusOK,
		"/read/" + items["Novel"] + "/file":                      http.StatusNotFound,
		"/read/" + items["Novel"] + "/asset/OEBPS/one.xhtml":     http.StatusOK,
		"/read/" + items["Novel"] + "/asset/OEBPS/missing.xhtml": http.StatusNotFound,
		"/read/" + items["Novel"] + "/asset/%2e%2e%2fsecret":     http.StatusNotFound,
		"/read/missing/asset/OEBPS/one.xhtml":                    http.StatusNotFound,
	} {
		response := apiCall(t, handler, "", http.MethodGet, path, nil)
		if response.Code != status {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
		if status == http.StatusOK && (response.Header().Get("X-Frame-Options") != "SAMEORIGIN" || !strings.Contains(response.Header().Get("Content-Security-Policy"), "frame-ancestors 'self'")) {
			t.Fatalf("GET %s cannot be embedded by the same-origin reader: %v", path, response.Header())
		}
	}
}

func apiItemsByTitle(t *testing.T, handler http.Handler) map[string]string {
	t.Helper()
	response := apiCall(t, handler, "", http.MethodGet, "/api/v1/library", nil)
	var library struct {
		Items []struct{ ID, Title string }
	}
	mustJSON(t, response, &library)
	result := make(map[string]string, len(library.Items))
	for _, item := range library.Items {
		result[item.Title] = item.ID
	}
	return result
}
