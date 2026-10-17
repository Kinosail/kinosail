package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkCatalogListingWireBytes(b *testing.B) {
	items := snapshotFixtureItems(snapshotBenchmarkCount)
	fixture := newSnapshotFixture(b, items)
	show := library.ShowID(items[snapshotBenchmarkCount/2+5].Show)
	for _, target := range []struct{ name, path string }{
		{"library-page", "/api/v1/library"},
		{"shows", "/api/v1/shows"},
		{"show", "/api/v1/shows/" + show},
		{"albums", "/api/v1/albums"},
	} {
		for _, encoding := range []string{"identity", "gzip"} {
			b.Run(target.name+"/"+encoding, func(b *testing.B) {
				request := snapshotRequest(b.Context(), http.MethodGet, target.path, viewerProfile{ID: "owner", Owner: true})
				request.Header.Set("Accept-Encoding", encoding)
				b.ReportAllocs()
				wire := 0
				for b.Loop() {
					response := httptest.NewRecorder()
					fixture.mux.ServeHTTP(response, request)
					if response.Code != http.StatusOK {
						b.Fatalf("%s = %d", target.path, response.Code)
					}
					wire = response.Body.Len()
				}
				b.ReportMetric(float64(wire), "wire-bytes/op")
			})
		}
	}
}
