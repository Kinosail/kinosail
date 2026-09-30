package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

func BenchmarkNativeCatalogNavigation(b *testing.B) {
	for _, count := range []int{10_000, 100_000} {
		b.Run(strconv.Itoa(count), func(b *testing.B) {
			index := memoryLibraryIndex(benchmarkLibraryItems(count), true)
			progress, lists := newProgressStore(""), newListStore("")
			handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
				return browseLibrary(request, index, progress, lists)
			}, progress.ClientItem)
			for name, target := range map[string]string{
				"browse": "/api/v1/library?view=movies",
				"search": "/api/v1/library?view=movies&q=Movie+" + strconv.Itoa(count-1),
			} {
				b.Run(name, func(b *testing.B) {
					request := ownerRequest(target)
					b.ReportAllocs()
					responseBytes := 0
					for b.Loop() {
						response := httptest.NewRecorder()
						handler(response, request)
						if response.Code != http.StatusOK {
							b.Fatal(response.Code)
						}
						responseBytes = response.Body.Len()
					}
					b.ReportMetric(float64(responseBytes), "response-bytes")
				})
			}
		})
	}
}
