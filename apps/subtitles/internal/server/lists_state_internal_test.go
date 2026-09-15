package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func failingListStore() *listStore {
	return &listStore{
		file: "lists.json", playlistsFile: "playlists.json", playlistOrderFile: "playlist_order.json", smartFile: "smart_playlists.json",
		values:        map[string]bool{"viewer:aaaaaaaaaaaaaaaa": true},
		playlists:     map[string]map[string]bool{"viewer:Queue": {"aaaaaaaaaaaaaaaa": true, "bbbbbbbbbbbbbbbb": true}},
		playlistOrder: map[string][]string{"viewer:Queue": {"aaaaaaaaaaaaaaaa", "bbbbbbbbbbbbbbbb"}},
		smart:         map[string]playlistRule{"viewer:Recent": {Kind: "video", Sort: "added"}},
		persist:       func(string, any) error { return errors.New("blocked") },
	}
}

func TestListMutationsDoNotChangeMemoryWhenPersistenceFails(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	operations := map[string]func(*listStore) error{
		"create": func(store *listStore) error { return store.Create(ctx, "viewer", "New") },
		"create smart": func(store *listStore) error {
			return store.CreateSmart(ctx, "viewer", "Smart", playlistRule{Sort: "title"})
		},
		"add item": func(store *listStore) error {
			return store.SetPlaylist(ctx, "viewer", "Queue", "cccccccccccccccc", true)
		},
		"order": func(store *listStore) error {
			return store.Order(ctx, "viewer", "Queue", []string{"bbbbbbbbbbbbbbbb", "aaaaaaaaaaaaaaaa"})
		},
		"delete":            func(store *listStore) error { return store.DeletePlaylist(ctx, "viewer", "Queue") },
		"set listed":        func(store *listStore) error { return store.SetListed(ctx, "viewer", "bbbbbbbbbbbbbbbb", true) },
		"create collection": func(store *listStore) error { return store.createCollection(ctx, "New") },
		"set collection": func(store *listStore) error {
			return store.setCollection(ctx, "Curated", library.Item{ID: "cccccccccccccccc", Collection: "Curated"}, true)
		},
		"delete collection": func(store *listStore) error {
			return store.deleteCollection(ctx, "Curated", []library.Item{{ID: "cccccccccccccccc", Collection: "Curated"}})
		},
	}
	for name, operation := range operations {
		t.Run(name, func(t *testing.T) {
			store := failingListStore()
			before := store.storage().State()
			if err := operation(store); err == nil {
				t.Fatal("mutation succeeded with failed persistence")
			}
			if after := store.storage().State(); !reflect.DeepEqual(after, before) {
				t.Fatalf("failed mutation changed memory: before=%#v after=%#v", before, after)
			}
		})
	}
}

func TestPersistedListAndProgressValidationIsBounded(t *testing.T) {
	t.Parallel()
	if catalog.ValidateListState(catalog.ListState{Values: map[string]bool{"": true}, Playlists: map[string]map[string]bool{}, PlaylistOrder: map[string][]string{}, Smart: map[string]playlistRule{}}) == nil {
		t.Fatal("invalid persisted list key was accepted")
	}
	if catalog.ValidateListState(catalog.ListState{Values: map[string]bool{}, Playlists: map[string]map[string]bool{}, PlaylistOrder: map[string][]string{"viewer:Queue": {"same", "same"}}, Smart: map[string]playlistRule{}}) == nil {
		t.Fatal("duplicate persisted playlist order was accepted")
	}
	if catalog.ValidateStoredProgress(map[string]playbackState{"viewer:item": {Seconds: -1}}) == nil {
		t.Fatal("invalid persisted progress was accepted")
	}
}

func TestProgressMutationDoesNotChangeMemoryWhenPersistenceFails(t *testing.T) {
	t.Parallel()
	store := newProgressStore(t.TempDir())
	store.Replace(map[string]playbackState{"viewer:item": {Seconds: 10}})
	store.SetPersistence(func(string, any) error { return errors.New("blocked") })
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "viewer"}), http.MethodPut, "/", nil)
	if _, err := store.SetRevision(request, "item", 20, nil, "", 0); err == nil {
		t.Fatal("progress mutation succeeded with failed persistence")
	}
	if state := store.Get(request, "item"); state.Seconds != 10 {
		t.Fatalf("failed progress mutation changed memory: %#v", state)
	}
}

func TestProgressMutationRejectsInvalidDurableStateBeforePersistence(t *testing.T) {
	t.Parallel()
	calls := 0
	store := newProgressStore(t.TempDir())
	store.SetPersistence(func(string, any) error {
		calls++
		return nil
	})
	request := httptest.NewRequestWithContext(withViewer(t.Context(), viewerProfile{ID: "viewer"}), http.MethodPut, "/", nil)
	if _, err := store.SetRevision(request, "item", 42, nil, strings.Repeat("s", 129), 1); !errors.Is(err, errInvalidProgressState) {
		t.Fatalf("oversized progress session error = %v", err)
	}
	if calls != 0 || store.Len() != 0 {
		t.Fatalf("invalid progress caused side effects: calls=%d records=%d", calls, store.Len())
	}
}
