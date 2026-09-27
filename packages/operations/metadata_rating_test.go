package operations

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func TestRefreshMetadataChecksExistingMoviesForMissingCertificationOnce(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	artwork := filepath.Join(directory, "movie.jpg")
	if err := os.WriteFile(artwork, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}
	items := []library.Item{{ID: "old", Kind: "video", Artwork: artwork}, {ID: "checked", Kind: "video", Artwork: artwork}, {ID: "rated", Kind: "video", Artwork: artwork, Rating: "PG-13"}}
	records := map[string]metadata.Record{
		"old":     {Title: "Old", CastFetched: true, BackdropChecked: true},
		"checked": {Title: "Checked", CastFetched: true, BackdropChecked: true, RatingChecked: true},
		"rated":   {Title: "Rated", CastFetched: true, BackdropChecked: true},
	}
	config := metadataFixture(items, records)
	config.Configured = true
	var resolved []string
	config.Resolve = func(_ context.Context, item library.Item) (metadata.Result, error) {
		resolved = append(resolved, item.ID)
		return metadata.Result{Record: metadata.Record{Title: item.ID, Rating: "PG-13", RatingChecked: true}}, nil
	}
	if err := RefreshMetadata(t.Context(), config); err != nil || strings.Join(resolved, ",") != "old" {
		t.Fatalf("certification refresh = %v, resolved=%v", err, resolved)
	}
}
