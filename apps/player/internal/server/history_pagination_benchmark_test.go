package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func BenchmarkPlaybackHistoryAPI(b *testing.B) {
	items := snapshotFixtureItems(snapshotBenchmarkCount)
	fixture := newSnapshotFixture(b, items)
	owner := viewerProfile{ID: "owner", Owner: true}
	for _, item := range items {
		if err := fixture.progress.Set(snapshotRequest(b.Context(), http.MethodPut, "/", owner), item.ID, 30, nil); err != nil {
			b.Fatal(err)
		}
	}
	for _, test := range []struct{ name, target string }{
		{"unpaged", "/api/v1/history"},
		{"first-page", "/api/v1/history?limit=50"},
		{"deep-page", "/api/v1/history?offset=19800&limit=200"},
	} {
		b.Run(test.name, func(b *testing.B) {
			request := snapshotRequest(b.Context(), http.MethodGet, test.target, owner)
			size := 0
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				fixture.mux.ServeHTTP(response, request)
				if response.Code != http.StatusOK {
					b.Fatalf("%s = %d", test.target, response.Code)
				}
				size = response.Body.Len()
			}
			b.ReportMetric(float64(size), "response-bytes")
		})
	}
}
