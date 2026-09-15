package catalog_test

import (
	"reflect"
	"slices"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func readStorage(t *testing.T, state *catalog.ListState) catalog.ListStorage {
	t.Helper()
	return catalog.ListStorage{Playlists: &state.Playlists, PlaylistOrder: &state.PlaylistOrder, Smart: &state.Smart, Persist: func(string, any) error {
		t.Fatal("read attempted persistence")
		return nil
	}}
}

func TestPlaylistNamesKeepViewerAndCollectionBoundaries(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{
		Playlists: map[string]map[string]bool{"viewer:Zebra": {}, "viewer:collection:Films": {}, "viewer2:Private": {}, "other:Secret": {}},
		Smart:     map[string]catalog.PlaylistRule{"viewer:Audio": {Kind: "audio", Sort: "title"}, "other:Hidden": {Sort: "title"}},
	}
	storage := readStorage(t, &state)
	if got := storage.PlaylistNames("viewer"); !slices.Equal(got, []string{"Audio", "Zebra"}) {
		t.Fatalf("names = %v", got)
	}
	if got := storage.EditablePlaylistNames("viewer"); !slices.Equal(got, []string{"Zebra"}) {
		t.Fatalf("editable names = %v", got)
	}
	if got := storage.PlaylistNames("missing"); got == nil || len(got) != 0 {
		t.Fatalf("missing viewer names = %v", got)
	}
	if got := storage.EditablePlaylistNames("missing"); got == nil || len(got) != 0 {
		t.Fatalf("missing viewer editable names = %v", got)
	}
}

func TestManualPlaylistKeepsOrderAndVisibleMembership(t *testing.T) {
	t.Parallel()
	state := catalog.ListState{
		Playlists:     map[string]map[string]bool{"viewer:Queue": {"a": true, "b": true, "c": true, "hidden": true, "excluded": false}, "other:Queue": {"excluded": true}},
		PlaylistOrder: map[string][]string{"viewer:Queue": {"hidden", "c", "c", "missing", "excluded", "b"}},
	}
	before := catalog.CloneListState(state.Values, state.Playlists, state.PlaylistOrder, state.Smart)
	storage := readStorage(t, &state)
	items := []library.Item{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "excluded"}}
	if got := storage.Playlist("viewer", "Queue", items); !reflect.DeepEqual(got, []library.Item{items[2], items[1], items[0]}) {
		t.Fatalf("manual members = %v", got)
	}
	if got := storage.Playlist("missing", "Queue", items); got == nil || len(got) != 0 {
		t.Fatalf("missing viewer members = %v", got)
	}
	if got := storage.Playlist("viewer", "missing", items); got == nil || len(got) != 0 {
		t.Fatalf("missing playlist members = %v", got)
	}
	after := catalog.CloneListState(state.Values, state.Playlists, state.PlaylistOrder, state.Smart)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("playlist read changed stored state")
	}
}

func TestSmartPlaylistUsesCanonicalSearchKindsAndSort(t *testing.T) {
	t.Parallel()
	items := []library.Item{
		{ID: "song", Title: "Zebra", Kind: "audio", Year: "2020"},
		{ID: "book", Title: "Alpha", Kind: "audiobook", Year: "2022"},
		{ID: "film", Title: "Bravo", Kind: "video", Year: "2021"},
	}
	for _, test := range []struct {
		name string
		rule catalog.PlaylistRule
		want []string
	}{
		{"audio includes audiobooks", catalog.PlaylistRule{Kind: "audio", Sort: "title"}, []string{"book", "song"}},
		{"specific kind", catalog.PlaylistRule{Kind: "video", Sort: "title"}, []string{"film"}},
		{"query", catalog.PlaylistRule{Query: "ALPHA", Sort: "title"}, []string{"book"}},
		{"all kinds sorted", catalog.PlaylistRule{Sort: "year"}, []string{"book", "film", "song"}},
		{"no match", catalog.PlaylistRule{Query: "missing", Sort: "title"}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := catalog.ListState{Smart: map[string]catalog.PlaylistRule{"viewer:Smart": test.rule}, Playlists: map[string]map[string]bool{"viewer:Smart": {"film": true}}}
			storage := readStorage(t, &state)
			input := slices.Clone(items)
			got := storage.Playlist("viewer", "Smart", input)
			if !reflect.DeepEqual(input, items) {
				t.Fatal("smart playlist changed caller item order")
			}
			ids := make([]string, 0, len(got))
			for _, item := range got {
				ids = append(ids, item.ID)
			}
			if !slices.Equal(ids, test.want) {
				t.Fatalf("members = %v, want %v", ids, test.want)
			}
		})
	}
}
