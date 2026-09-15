package catalog_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestCollectionReadsKeepDefaultsExclusionsAndCustomMembers(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{Playlists: map[string]map[string]bool{
		"collection:Films":   {"!excluded": true, "custom": true},
		"collection:Deleted": {"!collection": true},
		"collection:Empty":   {},
		"viewer:Private":     {},
	}}
	storage := readStorage(t, &state)
	items := []library.Item{{ID: "default", Collection: "Films"}, {ID: "excluded", Collection: "Films"}, {ID: "custom"}, {ID: "deleted", Collection: "Deleted"}, {ID: "other"}}
	if got := storage.CollectionNames(items); !slices.Equal(got, []string{"Empty", "Films"}) {
		t.Fatalf("names = %v", got)
	}
	if got := storage.Collection("Films", items); !reflect.DeepEqual(got, []library.Item{items[0], items[2]}) {
		t.Fatalf("members = %v", got)
	}
	if got := storage.Collection("Missing", items); got == nil || len(got) != 0 {
		t.Fatalf("missing collection = %v", got)
	}
}

func summaryItems() []library.Item {
	return []library.Item{
		{ID: "a", Title: "Alpha", Collection: "Films", Artwork: "poster"},
		{ID: "b", Title: "Beta", Collection: "Films", ShowArtwork: "show poster"},
		{ID: "c", Title: "Gamma", Collection: "Films"},
		{ID: "d", Title: "Delta", Collection: "Films"},
		{ID: "e", Title: "Epsilon", Collection: "Films", Artwork: "fifth poster"},
	}
}

func TestCollectionSummariesPreserveProjectionAndSearch(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{Playlists: map[string]map[string]bool{"collection:Empty": {}, "collection:Single": {"a": true}}}
	storage := readStorage(t, &state)
	summaries := storage.CollectionSummaries(summaryItems(), "")
	if len(summaries) != 3 {
		t.Fatalf("summaries = %#v", summaries)
	}
	items := summaryItems()
	want := []catalog.CollectionSummary{
		{Name: "Empty", Source: "custom", ItemLabel: "0 items", ArtworkIDs: []string{}},
		{Name: "Films", Source: "default", ItemCount: 5, ItemLabel: "5 items", ArtworkIDs: []string{"a", "b"}, Preview: items[:4]},
		{Name: "Single", Source: "custom", ItemCount: 1, ItemLabel: "1 item", ArtworkIDs: []string{"a"}, Preview: items[:1]},
	}
	if !reflect.DeepEqual(summaries, want) {
		t.Fatalf("summaries = %#v, want %#v", summaries, want)
	}
}

func TestCollectionSummarySearchAndJSON(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{Playlists: map[string]map[string]bool{"collection:Empty": {}, "collection:Single": {"a": true}}}
	storage := readStorage(t, &state)
	if got := storage.CollectionSummaries(summaryItems(), "ALPHA"); len(got) != 2 {
		t.Fatalf("member search = %#v", got)
	}
	if got := storage.CollectionSummaries(summaryItems(), "fIlMs"); len(got) != 1 {
		t.Fatalf("name search = %#v", got)
	}
	if got := storage.CollectionSummaries(summaryItems(), "missing"); got == nil || len(got) != 0 {
		t.Fatalf("no match = %#v", got)
	}
	encoded, err := json.Marshal(storage.CollectionSummaries(summaryItems(), "Empty")[0])
	if err != nil || strings.Contains(string(encoded), "Preview") || strings.Contains(string(encoded), "ItemLabel") || !strings.Contains(string(encoded), `"artworkIds":[]`) {
		t.Fatalf("JSON = %s: %v", encoded, err)
	}
}

func TestPlaylistSummariesPreserveModeRulesAndPreview(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{
		Playlists: map[string]map[string]bool{"viewer:Manual": {"a": true, "b": true, "c": true, "d": true, "e": true}, "other:Private": {}},
		Smart:     map[string]catalog.PlaylistRule{"viewer:All": {}, "viewer:Filtered": {Query: "Alpha", Kind: "video"}},
	}
	storage := readStorage(t, &state)
	summaries := storage.PlaylistSummaries("viewer", summaryItems(), "")
	if len(summaries) != 3 {
		t.Fatalf("summaries = %#v", summaries)
	}
	all, filtered, manual := summaries[0], summaries[1], summaries[2]
	if all.Mode != "Smart" || all.Rule != "All media" {
		t.Fatalf("all = %#v", all)
	}
	if filtered.Mode != "Smart" || filtered.Rule != `matching "Alpha" · video` || filtered.ItemCount != 0 {
		t.Fatalf("filtered = %#v", filtered)
	}
	want := catalog.PlaylistSummary{Name: "Manual", Mode: "Manual", ItemCount: 5, ArtworkIDs: []string{"a", "b"}, Preview: summaryItems()[:4]}
	if !reflect.DeepEqual(manual, want) {
		t.Fatalf("manual = %#v", manual)
	}
	if got := storage.PlaylistSummaries("viewer", summaryItems(), "missing"); got == nil || len(got) != 0 {
		t.Fatalf("no match = %#v", got)
	}
}

func TestPlaylistSummarySearchContinuesAfterUnmatchedNames(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{Playlists: map[string]map[string]bool{"viewer:Alpha": {}, "viewer:Zulu": {"a": true}}}
	storage := readStorage(t, &state)
	got := storage.PlaylistSummaries("viewer", summaryItems(), "Zulu")
	if len(got) != 1 || got[0].Name != "Zulu" || got[0].ItemCount != 1 {
		t.Fatalf("search stopped before the matching playlist: %#v", got)
	}
}
