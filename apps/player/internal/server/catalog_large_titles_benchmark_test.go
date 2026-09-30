package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func BenchmarkNativeCatalogLargeTitles(b *testing.B) {
	for _, scenario := range []struct{ name, title string }{
		{"long-ascii", strings.Repeat("Long title. ", 100)},
		{"unicode-expansion", strings.Repeat("ﷺ", 150)},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			items := benchmarkLibraryItems(1000)
			for position := range items {
				items[position].Title = scenario.title
				items[position].Plot = strings.Repeat("A quiet journey. ", 120) + "Café"
			}
			index := memoryLibraryIndex(items, true)
			progress, lists := newProgressStore(""), newListStore("")
			handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
				return browseLibrary(request, index, progress, lists)
			}, progress.ClientItem)
			request := ownerRequest("/api/v1/library?view=movies&q=Absent+owl")
			ctx, cancel := context.WithCancel(request.Context())
			b.Cleanup(cancel)
			request = request.WithContext(ctx)
			responseBytes := 0
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				handler(response, request)
				if response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"total":0,`)) {
					b.Fatalf("wrong search result status=%d bytes=%d", response.Code, response.Body.Len())
				}
				responseBytes = response.Body.Len()
			}
			b.ReportMetric(float64(responseBytes), "response-bytes")
		})
	}
}
