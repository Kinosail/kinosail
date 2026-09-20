package catalog

import (
	"errors"
	"os"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func collectionMutationStorage() ListStorage {
	var loadError error
	state := ListState{}
	return ListStorage{Values: &state.Values, Playlists: &state.Playlists, PlaylistOrder: &state.PlaylistOrder, Smart: &state.Smart, LoadError: &loadError}
}

func TestCollectionMutationPreservesImportedExclusions(t *testing.T) {
	t.Parallel()
	storage := collectionMutationStorage()
	item := library.Item{ID: "movie", Collection: "Imported"}
	if err := storage.SetCollection(t.Context(), "Imported", item, false); err != nil {
		t.Fatal(err)
	}
	if !(*storage.Playlists)["collection:Imported"]["!movie"] {
		t.Fatal("imported exclusion was not retained")
	}
	if err := storage.SetCollection(t.Context(), "Imported", item, true); err != nil {
		t.Fatal(err)
	}
	if (*storage.Playlists)["collection:Imported"]["!movie"] || !(*storage.Playlists)["collection:Imported"]["movie"] {
		t.Fatal("explicit inclusion did not replace exclusion")
	}
}

func TestDeletedCollectionCannotAcceptMembersUntilRecreated(t *testing.T) {
	t.Parallel()
	storage := collectionMutationStorage()
	item := library.Item{ID: "movie", Collection: "Imported"}
	if err := storage.DeleteCollection(t.Context(), "Imported", []library.Item{item}); err != nil {
		t.Fatal(err)
	}
	if err := storage.SetCollection(t.Context(), "Imported", item, true); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("deleted collection mutation=%v", err)
	}
	if err := storage.CreateCollection(t.Context(), "Imported"); err != nil {
		t.Fatal(err)
	}
	if (*storage.Playlists)["collection:Imported"]["!collection"] {
		t.Fatal("recreated collection remains hidden")
	}
}

func TestCollectionNamesRejectInvalidCreationAndEscapeReadPaths(t *testing.T) {
	t.Parallel()
	storage := collectionMutationStorage()
	if err := storage.CreateCollection(t.Context(), "bad/name"); err == nil || len(*storage.Playlists) != 0 {
		t.Fatal("invalid creation changed state")
	}
	summary := CollectionSummary{Name: "Films/TV?all"}
	if got := summary.Path(); got != "/collection/Films%2FTV%3Fall" {
		t.Fatalf("path=%s", got)
	}
}
