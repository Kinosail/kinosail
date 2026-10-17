package servertest

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type wireLibrary struct {
	media, poster string
	ids           map[string]string
	direct        []byte
}

func (fixture AssetsFixture) CatalogListingsCompressWithoutChangingMediaOrPages(t *testing.T, listings ...string) {
	t.Helper()
	t.Parallel()
	populated := populateWireLibrary(t)
	handler := fixture.NewHandler(populated.media, t.TempDir(), false)
	populated.discover(t, handler)
	for _, listing := range listings {
		assertCompressedListing(t, handler, populated.resolve(listing))
	}
	if page := wireGet(t, handler, "/", http.Header{"Accept-Encoding": {"gzip"}}); page.Code != http.StatusOK || page.Header().Get("Content-Encoding") != "" || page.Body.Len() < 1024 {
		t.Fatalf("home page = %d %q, %d bytes", page.Code, page.Header().Get("Content-Encoding"), page.Body.Len())
	}
	media := "/media/" + populated.ids["movie"]
	whole := wireGet(t, handler, media, http.Header{"Accept-Encoding": {"gzip"}})
	partial := wireGet(t, handler, media, http.Header{"Accept-Encoding": {"gzip"}, "Range": {"bytes=2-9"}})
	if whole.Code != http.StatusOK || whole.Header().Get("Content-Encoding") != "" || !bytes.Equal(whole.Body.Bytes(), populated.direct) {
		t.Fatalf("direct media = %d %v %q", whole.Code, whole.Header(), whole.Body.Bytes())
	}
	if partial.Code != http.StatusPartialContent || partial.Header().Get("Content-Encoding") != "" || partial.Header().Get("Content-Range") != fmt.Sprintf("bytes 2-9/%d", len(populated.direct)) || !bytes.Equal(partial.Body.Bytes(), populated.direct[2:10]) {
		t.Fatalf("direct media range = %d %v %q", partial.Code, partial.Header(), partial.Body.Bytes())
	}
}

func (fixture AssetsFixture) ArtworkRevalidatesInsteadOfRedownloading(t *testing.T, images ...string) {
	t.Helper()
	t.Parallel()
	populated := populateWireLibrary(t)
	handler := fixture.NewHandler(populated.media, t.TempDir(), false)
	populated.discover(t, handler)
	for _, image := range images {
		path := populated.resolve(image)
		first := wireGet(t, handler, path, nil)
		modified := first.Header().Get("Last-Modified")
		if first.Code != http.StatusOK || first.Header().Get("Cache-Control") != "private, no-cache" || modified == "" || first.Body.Len() == 0 {
			t.Fatalf("%s = %d cache %q modified %q", path, first.Code, first.Header().Get("Cache-Control"), modified)
		}
		unchanged := wireGet(t, handler, path, http.Header{"If-Modified-Since": {modified}})
		if unchanged.Code != http.StatusNotModified || unchanged.Body.Len() != 0 || unchanged.Header().Get("Cache-Control") != "private, no-cache" {
			t.Fatalf("%s revalidation = %d cache %q, %d bytes", path, unchanged.Code, unchanged.Header().Get("Cache-Control"), unchanged.Body.Len())
		}
		t.Logf("%s: first load %d bytes, revalidation %d bytes", image, first.Body.Len(), unchanged.Body.Len())
	}
	replacement := []byte("replacement poster bytes")
	later := time.Now().Add(time.Hour)
	if err := os.WriteFile(populated.poster, replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(populated.poster, later, later); err != nil {
		t.Fatal(err)
	}
	path := populated.resolve("/art/{movie}")
	changed := wireGet(t, handler, path, http.Header{"If-Modified-Since": {time.Now().UTC().Format(http.TimeFormat)}})
	if changed.Code != http.StatusOK || !bytes.Equal(changed.Body.Bytes(), replacement) {
		t.Fatalf("%s after replacement = %d %q", path, changed.Code, changed.Body.Bytes())
	}
}

func assertCompressedListing(t *testing.T, handler http.Handler, path string) {
	t.Helper()
	plain := wireGet(t, handler, path, nil)
	if plain.Code != http.StatusOK || plain.Header().Get("Content-Encoding") != "" || !strings.HasPrefix(plain.Header().Get("Content-Type"), "application/json") || plain.Body.Len() < 1024 {
		t.Fatalf("%s identity = %d %v, %d bytes", path, plain.Code, plain.Header(), plain.Body.Len())
	}
	compressed := wireGet(t, handler, path, http.Header{"Accept-Encoding": {"gzip, deflate, br"}})
	header := compressed.Header()
	if compressed.Code != http.StatusOK || header.Get("Content-Encoding") != "gzip" || !strings.Contains(strings.Join(header.Values("Vary"), ","), "Accept-Encoding") || header.Get("Cache-Control") != "no-store" || !strings.HasPrefix(header.Get("Content-Type"), "application/json") {
		t.Fatalf("%s gzip = %d %v", path, compressed.Code, header)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil || !bytes.Equal(decoded, plain.Body.Bytes()) || compressed.Body.Len()*3 > plain.Body.Len() {
		t.Fatalf("%s gzip decoded %d bytes from %d, identity %d: %v", path, len(decoded), compressed.Body.Len(), plain.Body.Len(), err)
	}
	t.Logf("%s: identity %d bytes, gzip %d bytes", path, plain.Body.Len(), compressed.Body.Len())
	for _, refused := range []http.Header{{"Accept-Encoding": {"identity"}}, {"Accept-Encoding": {"gzip;q=0, identity"}}, {"Accept-Encoding": {"gzip"}, "Range": {"bytes=0-99"}}, {"Accept-Encoding": {"gzip"}, "If-None-Match": {`"listing"`}}} {
		response := wireGet(t, handler, path, refused)
		if response.Code != http.StatusOK || response.Header().Get("Content-Encoding") != "" || !bytes.Equal(response.Body.Bytes(), plain.Body.Bytes()) {
			t.Fatalf("%s with %v = %d %v", path, refused, response.Code, response.Header())
		}
	}
}

func populateWireLibrary(t *testing.T) wireLibrary {
	t.Helper()
	media := t.TempDir()
	populated := wireLibrary{media: media, ids: map[string]string{}, direct: []byte("kinosail direct media bytes")}
	write := func(path string, data []byte) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for position := range 40 {
		base := filepath.Join(media, fmt.Sprintf("The Long Voyage Part %02d (1999)", position))
		write(base+".mp4", populated.direct)
		write(base+".jpg", bytes.Repeat([]byte("poster"), 8<<10))
		write(base+"-fanart.jpg", bytes.Repeat([]byte("backdrop"), 16<<10))
	}
	populated.poster = filepath.Join(media, "The Long Voyage Part 00 (1999).jpg")
	for group := range 30 {
		count := 1
		if group == 0 {
			count = 12
		}
		for number := range count {
			show := fmt.Sprintf("Harbor Lights %02d", group)
			write(filepath.Join(media, show, "Season 01", fmt.Sprintf("%s - S01E%02d.mkv", show, number+1)), []byte("episode"))
			album, base := fmt.Sprintf("Low Tide %02d", group), filepath.Join(media, "Tidewater", fmt.Sprintf("Low Tide %02d", group), fmt.Sprintf("%02d Current", number+1))
			write(base+".mp3", []byte("audio"))
			write(base+".nfo", fmt.Appendf(nil, "<song><title>Current %02d</title><artist>Tidewater</artist><album>%s</album><track>%d</track></song>", number+1, album, number+1))
		}
	}
	return populated
}

func (populated wireLibrary) discover(t *testing.T, handler http.Handler) {
	t.Helper()
	var catalog struct {
		Items []struct{ ID, Kind, Title, Show string } `json:"items"`
	}
	MustJSON(t, wireGet(t, handler, "/api/v1/library?q=Part%2000", nil), &catalog)
	for _, item := range catalog.Items {
		if item.Title == "The Long Voyage Part 00" {
			populated.ids["movie"] = item.ID
		}
	}
	if populated.ids["movie"] == "" {
		t.Fatalf("library did not expose the first movie: %+v", catalog.Items)
	}
	for kind, title := range map[string]string{"show": "Harbor Lights 00", "album": "Low Tide 00"} {
		var groups map[string][]struct{ ID, Title string }
		if response := wireGet(t, handler, "/api/v1/"+kind+"s", nil); response.Code == http.StatusOK && json.Unmarshal(response.Body.Bytes(), &groups) == nil {
			for _, group := range groups[kind+"s"] {
				if group.Title == title {
					populated.ids[kind] = group.ID
				}
			}
		}
	}
}

func (populated wireLibrary) resolve(path string) string {
	for name, id := range populated.ids {
		path = strings.ReplaceAll(path, "{"+name+"}", id)
	}
	return path
}

func wireGet(t *testing.T, handler http.Handler, path string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil)
	for name, values := range header {
		request.Header[name] = values
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
