package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
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

func BenchmarkNativeCatalogMetadataSearch(b *testing.B) {
	for name, plot := range map[string]string{
		"short-ascii":  strings.Repeat("A quiet journey. ", 10),
		"long-ascii":   strings.Repeat("A quiet journey. ", 120),
		"late-unicode": strings.Repeat("A quiet journey. ", 120) + "Café",
	} {
		b.Run(name, func(b *testing.B) {
			items := benchmarkLibraryItems(10_000)
			for position := range items {
				items[position].Plot = plot
				items[position].Genres = "Drama / Mystery"
				items[position].Director = "Alex North"
				items[position].Cast = []library.Person{{Name: "Sam Reed", Role: "Captain"}, {Name: "Morgan Vale", Role: "Guide"}}
			}
			index := memoryLibraryIndex(items, true)
			progress, lists := newProgressStore(""), newListStore("")
			handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
				return browseLibrary(request, index, progress, lists)
			}, progress.ClientItem)
			request := ownerRequest("/api/v1/library?view=movies&q=Movie+9999")
			b.ReportAllocs()
			responseBytes := 0
			for b.Loop() {
				response := httptest.NewRecorder()
				handler(response, request)
				if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"id":"9999"`) {
					b.Fatal("known-title search did not return its item")
				}
				responseBytes = response.Body.Len()
			}
			b.ReportMetric(float64(responseBytes), "response-bytes")
		})
	}
}
