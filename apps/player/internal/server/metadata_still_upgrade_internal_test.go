package server

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/operations"
)

func TestRefreshReplacesLegacyW780EpisodeStillsWithOriginals(t *testing.T) {
	for _, test := range []struct {
		name          string
		width, height int
		replaced      bool
	}{
		{name: "legacy w780 still", width: 780, height: 439, replaced: true},
		{name: "sharp still", width: 1920, height: 1080},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newMetadataStore(MetadataConfig{Token: "test-token"}, t.TempDir(), t.TempDir())
			poster, backdrop := store.artworkPath("tv-1"), store.artworkPath("tv-1-backdrop")
			writePreservedBackdrop(t, poster, backdropResolutionJPEG(t, 600, 900))
			writePreservedBackdrop(t, backdrop, backdropResolutionJPEG(t, 1920, 1080))
			prior := backdropResolutionJPEG(t, test.width, test.height)
			items, records := make([]library.Item, 0, 2), make(map[string]metadata.Record, 2)
			for episode, id := range []string{"episode-one", "episode-two"} {
				still := store.artworkPath(id)
				writePreservedBackdrop(t, still, prior)
				record := metadata.Record{Title: id, Artwork: still, ShowTitle: "Series", ShowArtwork: poster, ShowBackdrop: backdrop, BackdropChecked: true, CastFetched: true, RatingChecked: true}
				if err := store.set(id, record); err != nil {
					t.Fatal(err)
				}
				records[id] = record
				items = append(items, library.Item{ID: id, Kind: "video", Title: id, Show: "Series", Season: 1, Episode: episode + 1, Library: "shows", Artwork: still, ShowArtwork: poster})
			}
			sharp := backdropResolutionJPEG(t, 1920, 1080)
			requested := map[string]int{}
			store.client = &http.Client{Transport: backdropResolutionTransport(func(request *http.Request) (*http.Response, error) {
				requested[request.URL.String()]++
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader(sharp))}, nil
			})}
			resolved := 0
			result := func(item library.Item) metadata.Result {
				resolved++
				record := records[item.ID]
				return metadata.Result{Record: record, Images: []metadata.Image{{Source: "/" + item.ID + ".jpg", Target: record.Artwork, Still: true}}}
			}
			config := metadataRefreshConfig(store, items, func(context.Context) error { return nil })
			config.Resolve = func(_ context.Context, item library.Item) (metadata.Result, error) { return result(item), nil }
			config.ResolveEpisode = func(_ context.Context, item library.Item, _ metadata.Record) (metadata.Result, error) {
				return result(item), nil
			}
			if err := operations.RefreshMetadata(t.Context(), config); err != nil {
				t.Fatal(err)
			}
			if want := map[bool]int{true: 2, false: 0}[test.replaced]; resolved != want {
				t.Fatalf("resolved %d times, want %d", resolved, want)
			}
			for _, item := range items {
				if test.replaced {
					if requested["https://image.tmdb.org/t/p/original/"+item.ID+".jpg"] != 1 {
						t.Fatalf("requests = %v", requested)
					}
					assertBackdropResolutionBytes(t, item.Artwork, sharp)
				} else {
					assertBackdropResolutionBytes(t, item.Artwork, prior)
				}
			}
		})
	}
}
