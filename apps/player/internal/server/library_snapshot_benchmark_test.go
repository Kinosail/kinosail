package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

const snapshotBenchmarkCount = 20_000

func BenchmarkLibrarySnapshotRequests(b *testing.B) {
	items := snapshotFixtureItems(snapshotBenchmarkCount)
	middle := snapshotBenchmarkCount / 2
	fixture := newSnapshotFixture(b, items, middle, middle+5, middle+9)
	movie, episode, track := items[middle], items[middle+5], items[middle+9]
	show := library.ShowID(episode.Show)
	for _, test := range []struct{ name, target string }{
		{"watch-episode", "/watch/" + episode.ID},
		{"watch-movie", "/watch/" + movie.ID},
		{"watch-track", "/watch/" + track.ID},
		{"show-page", "/show/" + show},
		{"api-show", "/api/v1/shows/" + show},
		{"home-landing", "/"},
		{"jellyfin-items-page", "/Items?startIndex=10000&limit=50"},
		{"jellyfin-items-search", "/Items?searchTerm=movie%20001&limit=50"},
		{"jellyfin-item", "/Items/" + movie.ID + "0000000000000000"},
		{"jellyfin-series-image", "/Items/" + show + "0000000000000000/Images/Primary"},
	} {
		for _, profile := range []viewerProfile{{ID: "owner", Owner: true}, {ID: "family", Rating: "teen", Libraries: []string{"all"}}} {
			b.Run(fmt.Sprintf("%s/owner=%t", test.name, profile.Owner), func(b *testing.B) {
				request := snapshotRequest(b.Context(), http.MethodGet, test.target, profile)
				b.ReportAllocs()
				for b.Loop() {
					response := httptest.NewRecorder()
					fixture.mux.ServeHTTP(response, request)
					if response.Code != http.StatusOK {
						b.Fatalf("%s = %d", test.target, response.Code)
					}
				}
			})
		}
	}
}
