package jellyfincompat

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkItemsListingWireBytes(b *testing.B) {
	base := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	items := make([]library.Item, 0, 2000)
	for position := range cap(items) {
		items = append(items, library.Item{ID: fmt.Sprintf("%016x", position), Kind: "video", Title: fmt.Sprintf("Movie %04d", position), Year: "2024", Genres: "Drama · Mystery", Artwork: "movie.jpg", Container: "MP4", Added: base.Add(time.Duration(position) * time.Minute), ProviderIDs: map[string]string{"imdb": fmt.Sprintf("tt%07d", position)}})
	}
	catalog := fixtureCatalog(items)
	values := url.Values{"limit": {"2000"}}
	for _, encoding := range []string{"identity", "gzip"} {
		b.Run(encoding, func(b *testing.B) {
			request := httptest.NewRequestWithContext(b.Context(), http.MethodGet, "/Items?limit=2000", nil)
			request.Header.Set("Accept-Encoding", encoding)
			b.ReportAllocs()
			wire := 0
			for b.Loop() {
				page, err := catalog.Items(values)
				if err != nil {
					b.Fatal(err)
				}
				response := httptest.NewRecorder()
				Listing(response, request, page)
				wire = response.Body.Len()
			}
			b.ReportMetric(float64(wire), "wire-bytes/op")
		})
	}
}
