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

func TestRefreshReplacesLegacyW780BackdropsWithOriginals(t *testing.T) {
	for _, test := range []struct {
		name          string
		base          string
		width, height int
		replaced      bool
	}{
		{name: "legacy w780 default", base: "https://image.tmdb.org/t/p/w780", width: 780, height: 439, replaced: true},
		{name: "sharp default", base: "https://image.tmdb.org/t/p/w780", width: 1920, height: 1080},
		{name: "custom base", base: "https://images.example/custom", width: 780, height: 439},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newMetadataStore(MetadataConfig{Token: "test-token", ImageURL: test.base}, t.TempDir(), t.TempDir())
			poster, target := store.artworkPath("movie"), store.artworkPath("movie-backdrop")
			writePreservedBackdrop(t, poster, backdropResolutionJPEG(t, 600, 900))
			prior := backdropResolutionJPEG(t, test.width, test.height)
			writePreservedBackdrop(t, target, prior)
			record := metadata.Record{Title: "Movie", Artwork: poster, Backdrop: target, BackdropChecked: true, CastFetched: true, RatingChecked: true}
			if err := store.set("movie", record); err != nil {
				t.Fatal(err)
			}
			sharp := backdropResolutionJPEG(t, 3840, 2160)
			store.client = &http.Client{Transport: backdropResolutionTransport(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != "https://image.tmdb.org/t/p/original/backdrop.jpg" {
					t.Errorf("backdrop upgrade requested %q", request.URL.Redacted())
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader(sharp))}, nil
			})}
			resolved := 0
			config := upgradeRefreshConfig(store, library.Item{ID: "movie", Kind: "video", Title: "Movie", Artwork: poster, Rating: "PG"}, record, &resolved)
			if err := operations.RefreshMetadata(t.Context(), config); err != nil {
				t.Fatal(err)
			}
			if want := map[bool]int{true: 1, false: 0}[test.replaced]; resolved != want {
				t.Fatalf("resolved %d times, want %d", resolved, want)
			}
			if test.replaced {
				assertBackdropResolutionBytes(t, target, sharp)
			} else {
				assertBackdropResolutionBytes(t, target, prior)
			}
		})
	}
}

func upgradeRefreshConfig(store *metadataStore, item library.Item, record metadata.Record, resolved *int) operations.MetadataRefresh {
	config := metadataRefreshConfig(store, []library.Item{item}, func(context.Context) error { return nil })
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		*resolved++
		return metadata.Result{Record: record, Images: []metadata.Image{{Source: "/backdrop.jpg", Target: record.Backdrop, Backdrop: true}}}, nil
	}
	return config
}
