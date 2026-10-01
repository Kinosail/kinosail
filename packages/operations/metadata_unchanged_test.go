package operations

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

// Unchanged optional metadata must stop scan feedback. Restored artwork must
// still republish the library, and failed artwork must remain retryable.
func TestRefreshMetadataDoesNotRepublishUnchangedProviderResults(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		state              string
		updates, downloads int
		failed             bool
	}{{"unchanged", 0, 0, false}, {"repaired artwork", 1, 1, false}, {"failed artwork", 0, 1, true}} {
		t.Run(scenario.state, func(t *testing.T) {
			t.Parallel()
			verifyUnchangedMetadataResult(t, scenario.state, scenario.updates, scenario.downloads, scenario.failed)
		})
	}
}

func verifyUnchangedMetadataResult(t *testing.T, state string, want, wantDownloads int, failed bool) {
	t.Helper()
	artwork := filepath.Join(t.TempDir(), "poster.jpg")
	record := metadata.Record{Title: "Arrival", Artwork: artwork, BackdropChecked: true, RatingChecked: true}
	itemArtwork := ""
	if state == "unchanged" {
		itemArtwork = artwork
		if err := os.WriteFile(artwork, []byte("poster"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := metadataFixture([]library.Item{{ID: "movie", Kind: "video", Title: record.Title, Artwork: itemArtwork}}, map[string]metadata.Record{"movie": record})
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		return metadata.Result{Record: record, Images: []metadata.Image{{Target: artwork}}}, nil
	}
	failure := errors.New("artwork unavailable")
	downloaded := 0
	config.Download = func(context.Context, metadata.Result) (metadata.Record, error) {
		downloaded++
		if state == "failed artwork" {
			return record, failure
		}
		return record, os.WriteFile(artwork, []byte("poster"), 0o600)
	}
	stored, refreshed := 0, 0
	config.Store = func(map[string]metadata.Record) error { stored++; return nil }
	config.RefreshLibrary = func(context.Context) error { refreshed++; return nil }
	err := RefreshMetadata(t.Context(), config)
	if stored != want || refreshed != want {
		t.Fatalf("unchanged %s persisted %d times and refreshed %d times, want %d", state, stored, refreshed, want)
	}
	if downloaded != wantDownloads {
		t.Fatalf("%s downloaded artwork %d times, want %d", state, downloaded, wantDownloads)
	}
	if (err != nil) != failed {
		t.Fatalf("%s error = %v", state, err)
	}
}

func TestRefreshMetadataPublishesUnchangedRecordMissingFromIndex(t *testing.T) {
	t.Parallel()
	record := metadata.Record{Title: "Arrival"}
	config := metadataFixture([]library.Item{{ID: "movie", Kind: "video"}}, map[string]metadata.Record{"movie": record})
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		return metadata.Result{Record: record}, nil
	}
	refreshed := 0
	config.RefreshLibrary = func(context.Context) error { refreshed++; return nil }
	if err := RefreshMetadata(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	if refreshed != 1 {
		t.Fatalf("metadata persisted before its library publication refreshed %d times, want 1", refreshed)
	}
}

func TestRefreshMetadataPublishesItsResultWhenConcurrentStoreChanges(t *testing.T) {
	t.Parallel()
	record := metadata.Record{Title: "Arrival"}
	records := make(map[string]metadata.Record)
	config := metadataFixture([]library.Item{{ID: "movie", Kind: "video"}}, records)
	config.Resolve = func(context.Context, library.Item) (metadata.Result, error) {
		// Another refresh persists before publishing its index snapshot.
		records["movie"] = record
		return metadata.Result{Record: record}, nil
	}
	stored, refreshed := 0, 0
	config.Store = func(map[string]metadata.Record) error { stored++; return nil }
	config.RefreshLibrary = func(context.Context) error { refreshed++; return nil }
	if err := RefreshMetadata(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	if stored != 1 || refreshed != 1 {
		t.Fatalf("concurrent metadata result persisted %d times and refreshed %d times, want 1", stored, refreshed)
	}
}
