package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

// Index construction is outside the timed request; each request starts without a
// reusable order. Include a sparse view to detect sorting unrelated content.
func BenchmarkNativeCatalogColdBrowse(b *testing.B) {
	for _, view := range []string{"all", "movies", "books"} {
		b.Run(view, func(b *testing.B) {
			items := benchmarkLibraryItems(100_000)
			items[0].Kind = "book"
			index := memoryLibraryIndex(items, true)
			progress, lists := newProgressStore(""), newListStore("")
			handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
				return browseLibrary(request, index, progress, lists)
			}, progress.ClientItem)
			request := ownerRequest("/api/v1/library?view=" + view)
			b.ReportAllocs()
			responseBytes := 0
			for b.Loop() {
				b.StopTimer()
				index = memoryLibraryIndex(items, true)
				b.StartTimer()
				response := httptest.NewRecorder()
				handler(response, request)
				if response.Code != http.StatusOK {
					b.Fatal(response.Code)
				}
				responseBytes = response.Body.Len()
				b.StopTimer()
				assertColdCatalogPage(b, view, response)
				b.StartTimer()
			}
			b.ReportMetric(float64(responseBytes), "response-bytes")
		})
	}
}

func assertColdCatalogPage(b *testing.B, view string, response *httptest.ResponseRecorder) {
	b.Helper()
	var body struct {
		Total int
		Items []struct{ ID string }
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		b.Fatal(err)
	}
	wantTotal := map[string]int{"all": 100_000, "movies": 99_999, "books": 1}[view]
	if body.Total != wantTotal || len(body.Items) != min(100, wantTotal) || view == "books" && body.Items[0].ID != "0" {
		b.Fatalf("view=%s total=%d items=%v", view, body.Total, body.Items)
	}
}
