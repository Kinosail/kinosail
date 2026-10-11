package server

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/operations"
)

// The public refresh journey uses a loopback image base, which must keep its
// existing behavior. This canonical-transport integration covers partial bulk
// persistence, including shared show propagation, without provider credentials.
func TestCanonicalBackdropBulkFailureKeepsCommittedMovieAndShowReferences(t *testing.T) {
	for _, test := range []struct {
		name         string
		show, cancel bool
	}{{name: "movie"}, {name: "show", show: true}, {name: "cancelled movie", cancel: true}} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			cache, data := t.TempDir(), t.TempDir()
			store := newMetadataStore(MetadataConfig{}, data, cache)
			target := store.artworkPath("existing-backdrop")
			prior := backdropResolutionJPEG(t, 780, 439)
			writePreservedBackdrop(t, target, prior)
			items := []library.Item{{ID: "first", Kind: "video", Title: "First"}}
			old := metadata.Record{Title: "First", Backdrop: target, BackdropChecked: true}
			if test.show {
				items[0].Show = "Series"
				items = append(items, library.Item{ID: "second", Kind: "video", Title: "Second", Show: "Series"})
				old.Backdrop, old.ShowTitle, old.ShowBackdrop = "", "Series", target
			}
			for _, item := range items {
				record := old
				record.Title = item.Title
				if err := store.set(item.ID, record); err != nil {
					t.Fatal(err)
				}
			}
			store.client = &http.Client{Transport: backdropResolutionTransport(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != "https://image.tmdb.org/t/p/original/backdrop.jpg" {
					t.Error("bulk backdrop did not use canonical original variant")
				}
				if test.cancel {
					cancel()
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"image/jpeg"}}, Body: io.NopCloser(bytes.NewReader([]byte("rejected replacement")))}, nil
			})}
			var displayed []library.Item
			refreshes := 0
			config := operations.MetadataRefresh{
				Available: true, Configured: true, Items: items,
				Record: func(id string) (metadata.Record, bool) {
					store.mu.RLock()
					defer store.mu.RUnlock()
					record, found := store.records[id]
					return record, found
				},
				Resolve: func(context.Context, library.Item) (metadata.Result, error) {
					record := old
					record.Title, record.CastFetched = "Updated first", true
					return metadata.Result{Record: record, Images: []metadata.Image{{Source: "/backdrop.jpg", Target: target, Backdrop: true, Show: test.show}}}, nil
				},
				ResolveEpisode: func(context.Context, library.Item, metadata.Record) (metadata.Result, error) {
					return metadata.Result{}, errors.New("existing episode should retain its metadata")
				},
				Download: store.downloadMetadataImages, Store: store.setAll,
				RefreshLibrary: func(context.Context) error {
					refreshes++
					displayed = store.apply(append([]library.Item(nil), items...))
					return nil
				},
			}
			wantRefreshes := 1
			if test.cancel {
				wantRefreshes = 0
			}
			err := operations.RefreshMetadata(ctx, config)
			if err == nil || refreshes != wantRefreshes || test.cancel && !errors.Is(err, context.Canceled) {
				t.Fatalf("partial/cancelled refresh outcome: refreshes=%d err=%v", refreshes, err)
			}
			reloaded := newMetadataStore(MetadataConfig{}, data, cache)
			if reloaded.err != nil {
				t.Fatal(reloaded.err)
			}
			if test.cancel {
				if reloaded.records["first"].Title != "First" {
					t.Fatal("cancellation committed partial metadata")
				}
				displayed = reloaded.apply(append([]library.Item(nil), items...))
			}
			for _, item := range displayed {
				record := reloaded.records[item.ID]
				visible, committed := item.Backdrop, record.Backdrop
				if test.show {
					visible, committed = item.ShowBackdrop, record.ShowBackdrop
				}
				if visible != target || committed != target || record.BackdropChecked != test.cancel {
					t.Fatalf("%s lost its visible/committed backdrop or stopped retries", item.ID)
				}
			}
			if len(displayed) != len(items) {
				t.Fatal("partial refresh did not republish every grouped item")
			}
			got, err := os.ReadFile(target)
			if err != nil || !bytes.Equal(got, prior) {
				t.Fatal("rejected bulk replacement changed the existing derivative")
			}
		})
	}
}

func TestCanonicalBackdropFailureDoesNotRetainUnusableCacheOrChangeCustomPolicy(t *testing.T) {
	prior := backdropResolutionJPEG(t, 780, 439)
	for _, test := range []struct {
		name, base string
		data       []byte
		expired    bool
	}{
		{name: "missing"},
		{name: "malformed", data: []byte("invalid prior bytes")},
		{name: "expired", data: prior, expired: true},
		{name: "custom unchanged", base: "https://images.example/custom", data: prior},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := newMetadataStore(MetadataConfig{ImageURL: test.base}, "", t.TempDir())
			target := store.artworkPath("backdrop")
			if test.data != nil {
				writePreservedBackdrop(t, target, test.data)
			}
			if test.expired {
				old := time.Now().Add(-metadata.ArtworkMaxAge - time.Hour)
				if err := os.Chtimes(target, old, old); err != nil {
					t.Fatal(err)
				}
			}
			store.client = &http.Client{Transport: backdropResolutionTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(nil))}, nil
			})}
			result := metadata.Result{Record: metadata.Record{Backdrop: target, BackdropChecked: true},
				Images: []metadata.Image{{Source: "/backdrop.jpg", Target: target, Backdrop: true}}}
			updated, err := store.downloadMetadataImages(t.Context(), result)
			if err == nil || updated.Backdrop != "" || updated.BackdropChecked {
				t.Fatal("failed refresh retained unusable artwork or changed custom-provider policy")
			}
		})
	}
}

func writePreservedBackdrop(t *testing.T, target string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
