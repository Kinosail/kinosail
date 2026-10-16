package dlna

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func browseEnvelope(object, flag string, start, count int) string {
	return fmt.Sprintf(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:Browse xmlns:u="%s"><ObjectID>%s</ObjectID><BrowseFlag>%s</BrowseFlag><Filter>*</Filter><StartingIndex>%d</StartingIndex><RequestedCount>%d</RequestedCount><SortCriteria></SortCriteria></u:Browse></s:Body></s:Envelope>`, contentDirectory, object, flag, start, count)
}

func BenchmarkBrowsePage(b *testing.B) {
	root := b.TempDir()
	items := make([]library.Item, 20_000)
	for position := range items {
		path := filepath.Join(root, fmt.Sprintf("%05d.mp4", position))
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			b.Fatal(err)
		}
		items[position] = library.Item{ID: fmt.Sprintf("%016x", position), Kind: "video", Title: fmt.Sprintf("Movie & %05d", position), Container: "MP4", Path: path}
	}
	index := catalog.NewMemoryIndex(items, true)
	index.SetRoots([]catalog.ScanRoot{{Path: root, Namespace: "Movies"}})
	mux := http.NewServeMux()
	settings := SettingsFuncs{TokenFunc: func() string { return "secret" }, ServerNameFunc: func() string { return "Kinosail" }}
	if err := Register(mux, b.Context(), Config{Base: "invalid", Settings: settings, Library: index, NotFound: http.NotFound}); err != nil {
		b.Fatal(err)
	}
	body := browseEnvelope("0", "BrowseDirectChildren", 10_000, 50)
	b.ReportAllocs()
	for b.Loop() {
		request := httptest.NewRequestWithContext(b.Context(), http.MethodPost, "/dlna/secret/content", strings.NewReader(body))
		request.Header.Set("SOAPAction", `"`+contentDirectory+`#Browse"`)
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, request)
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "Movie &amp;amp; 10000") {
			b.Fatalf("browse = %d", response.Code)
		}
	}
}
