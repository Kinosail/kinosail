package servertest

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// AssertCollectionSources checks custom and default collection projection parity.
func AssertCollectionSources(t *testing.T, summaries []catalog.CollectionSummary) {
	t.Helper()
	sources, counts := map[string]string{}, map[string]int{}
	for _, summary := range summaries {
		sources[summary.Name], counts[summary.Name] = summary.Source, summary.ItemCount
		if summary.Name == "Custom Set" && (len(summary.ArtworkIDs) != 1 || summary.ArtworkIDs[0] != "custom-item") {
			t.Fatalf("show artwork = %#v", summary.ArtworkIDs)
		}
	}
	if sources["Custom Set"] != "custom" || sources["Default Set"] != "default" || counts["Custom Set"] != 1 || counts["Default Set"] != 0 {
		t.Fatalf("sources = %#v, counts = %#v", sources, counts)
	}
	assertCollectionSourceJSON(t, summaries)
}

func assertCollectionSourceJSON(t *testing.T, summaries []catalog.CollectionSummary) {
	t.Helper()
	encoded, err := json.Marshal(summaries)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(encoded) || !bytes.Contains(encoded, []byte(`"source":"default"`)) || !bytes.Contains(encoded, []byte(`"source":"custom"`)) || !bytes.Contains(encoded, []byte(`"artworkIds":["custom-item"]`)) {
		t.Fatalf("invalid API projection: %q", encoded)
	}
}

// CollectionSources exercises the app collection adapter and its shared projection.
func CollectionSources(t *testing.T, create func(context.Context, string) error, set func(context.Context, string, library.Item, bool) error, summaries func([]library.Item, string) []catalog.CollectionSummary) {
	t.Helper()
	items := []library.Item{
		{ID: "default-item", Title: "First", Collection: "Default Set"},
		{ID: "custom-item", Title: "Second", ShowArtwork: "show-poster"},
	}
	if err := create(t.Context(), "Custom Set"); err != nil {
		t.Fatal(err)
	}
	if err := set(t.Context(), "Custom Set", items[1], true); err != nil {
		t.Fatal(err)
	}
	if err := set(t.Context(), "Default Set", items[0], false); err != nil {
		t.Fatal(err)
	}

	AssertCollectionSources(t, summaries(items, ""))
}
