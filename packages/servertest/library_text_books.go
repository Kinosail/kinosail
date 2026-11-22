package servertest

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func (fixture LibraryAPIFixture) ReaderServesPlainTextBooksInBookFolders(t *testing.T, itemsByTitle func(*testing.T, http.Handler) map[string]string) {
	media := t.TempDir()
	write := func(name string, content []byte) {
		t.Helper()
		path := filepath.Join(media, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("Books/Harbor Notes.txt", []byte("\xef\xbb\xbfHarbor Notes\r\n\r\nThe crew read <script>alert(1)</script> & café 漢字\r\nunder a second line\x00\x07\xff.\n\nLast paragraph."))
	var novel strings.Builder
	for line := 0; novel.Len() < 250<<10; line++ {
		fmt.Fprintf(&novel, "Line %06d — ünïcödé 漢字\n", line)
	}
	novel.WriteString(strings.Repeat("漢", 40<<10/3))
	novel.WriteString("\n")
	for line := 100000; novel.Len() < 600<<10; line++ {
		fmt.Fprintf(&novel, "Line %06d — ünïcödé 漢字\n", line)
	}
	write("Library/ebooks/Long Novel.txt", []byte(novel.String()))
	write("CC0-1.0.txt", []byte("license"))
	write("Movies/Release Notes.txt", []byte("notes"))
	write("Music/Album/Song.txt", []byte("lyrics"))
	huge := filepath.Join(media, "Books", "Huge Text.txt")
	write("Books/Huge Text.txt", nil)
	if err := os.Truncate(huge, 64<<20+1); err != nil {
		t.Fatal(err)
	}
	handler := fixture.NewHandler(media, "", false)
	items := itemsByTitle(t, handler)
	for _, stray := range []string{"CC0 1 0", "Release Notes", "Song"} {
		if _, found := items[stray]; found {
			t.Fatalf("text file outside a book folder became %q: %v", stray, items)
		}
	}
	notes, novelID, hugeID := items["Harbor Notes"], items["Long Novel"], items["Huge Text"]
	if notes == "" || novelID == "" || hugeID == "" {
		t.Fatalf("text books missing from library: %v", items)
	}

	AssertAPIBody(t, APICall(t, handler, "", http.MethodGet, "/api/v1/books/"+notes+"/reader", nil), http.StatusOK, `"type":"epub"`, `"/read/`+notes+`/asset/part-1.html"`)
	part := APICall(t, handler, "", http.MethodGet, "/read/"+notes+"/asset/part-1.html", nil)
	AssertAPIBody(t, part, http.StatusOK, "<p>Harbor Notes</p>", "&lt;script&gt;alert(1)&lt;/script&gt; &amp; café 漢字<br>under a second line\uFFFD.</p>", "<p>Last paragraph.</p>")
	body := part.Body.String()
	if strings.Contains(body, "<script>alert") || strings.ContainsAny(body, "\x00\x07\r\ufeff") {
		t.Fatalf("text part kept unsafe or raw bytes: %q", body)
	}
	if part.Header().Get("Content-Type") != "text/html; charset=utf-8" || !strings.Contains(part.Header().Get("Content-Security-Policy"), "script-src 'none'") || part.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("text part headers = %v", part.Header())
	}
	AssertAPIBody(t, APICall(t, handler, "", http.MethodGet, "/read/"+notes, nil), http.StatusOK, "Chapter 1 of 1", `src="/read/`+notes+`/asset/part-1.html"`)

	manifest := APICall(t, handler, "", http.MethodGet, "/api/v1/books/"+novelID+"/reader", nil)
	AssertAPIBody(t, manifest, http.StatusOK, `"type":"epub"`, "part-3.html")
	if strings.Contains(manifest.Body.String(), "part-4.html") {
		t.Fatalf("600 KiB novel should have three parts: %s", manifest.Body.String())
	}
	var joined strings.Builder
	for number := 1; number <= 3; number++ {
		response := APICall(t, handler, "", http.MethodGet, fmt.Sprintf("/read/%s/asset/part-%d.html", novelID, number), nil)
		if response.Code != http.StatusOK || response.Body.Len() > 8<<20 {
			t.Fatalf("part %d = %d with %d bytes", number, response.Code, response.Body.Len())
		}
		joined.WriteString(response.Body.String())
	}
	text := joined.String()
	if strings.ContainsRune(text, '\uFFFD') || strings.Count(text, "漢") != strings.Count(novel.String(), "漢") {
		t.Fatalf("parts split a character: replacement=%v han=%d want %d", strings.ContainsRune(text, '\uFFFD'), strings.Count(text, "漢"), strings.Count(novel.String(), "漢"))
	}
	for _, line := range regexp.MustCompile(`Line \d{6}`).FindAllString(novel.String(), -1) {
		if strings.Count(text, line+" ") != 1 {
			t.Fatalf("%s appears %d times across parts", line, strings.Count(text, line+" "))
		}
	}
	AssertAPIBody(t, APICall(t, handler, "", http.MethodPut, "/api/v1/books/"+novelID+"/reader/progress", map[string]any{"page": 3}), http.StatusOK, `"page":3`, `"total":3`)
	if response := APICall(t, handler, "", http.MethodPut, "/api/v1/books/"+novelID+"/reader/progress", map[string]any{"page": 4}); response.Code != http.StatusBadRequest {
		t.Fatalf("out-of-range progress = %d", response.Code)
	}
	AssertAPIBody(t, APICall(t, handler, "", http.MethodGet, "/api/v1/books/"+novelID+"/reader/progress", nil), http.StatusOK, `"page":3`)

	for _, name := range []string{"part-0.html", "part-4.html", "part-01.html", "part-1.htm", "part-1.html.txt", "Long%20Novel.txt", "..%2FCC0-1.0.txt"} {
		if response := APICall(t, handler, "", http.MethodGet, "/read/"+novelID+"/asset/"+name, nil); response.Code != http.StatusNotFound {
			t.Fatalf("asset %q = %d %q", name, response.Code, response.Body.String())
		}
	}
	if response := APICall(t, handler, "", http.MethodGet, "/api/v1/books/"+hugeID+"/reader", nil); response.Code != http.StatusNotFound {
		t.Fatalf("oversized text reader = %d", response.Code)
	}
	if response := APICall(t, handler, "", http.MethodGet, "/read/"+hugeID+"/asset/part-1.html", nil); response.Code != http.StatusNotFound {
		t.Fatalf("oversized text part = %d", response.Code)
	}
	if err := os.Remove(filepath.Join(media, "Books", "Harbor Notes.txt")); err != nil {
		t.Fatal(err)
	}
	if response := APICall(t, handler, "", http.MethodGet, "/read/"+notes+"/asset/part-1.html", nil); response.Code != http.StatusNotFound {
		t.Fatalf("removed text part = %d", response.Code)
	}
}
