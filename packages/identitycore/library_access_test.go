package identitycore

import (
	"errors"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

type libraryAccessIndex struct {
	items []library.Item
	err   error
	safe  bool
}

func (index libraryAccessIndex) Snapshot() ([]library.Item, error) {
	return append([]library.Item(nil), index.items...), index.err
}

func (index libraryAccessIndex) Find(id string) (library.Item, bool) {
	for _, item := range index.items {
		if item.ID == id {
			return item, true
		}
	}
	return library.Item{}, false
}

func (index libraryAccessIndex) Safe(string) bool { return index.safe }

func TestLibraryPolicyDefaultsFailClosed(t *testing.T) {
	t.Parallel()
	item := library.Item{Library: "Movies", Rating: "PG"}
	if CanView(Profile{}, item) {
		t.Fatal("new Viewer can access a library before it is granted")
	}
	viewer := Profile{Libraries: []string{"all"}}
	if !CanView(viewer, item) || CanView(viewer, library.Item{Library: "Movies", Rating: "R"}) || CanView(viewer, library.Item{Library: "Movies"}) {
		t.Fatal("new Viewer rating default is not family-safe")
	}
	policy := LibraryPolicy(Profile{Owner: true})
	if !policy.Owner || !policy.Allows(library.Item{}) {
		t.Fatal("Owner policy was not preserved")
	}
}

func TestViewerLibraryVisibilityPreservesSnapshotAndSafetyBoundaries(t *testing.T) {
	t.Parallel()
	warning := errors.New("snapshot warning")
	index := libraryAccessIndex{items: []library.Item{
		{ID: "allowed", Library: "Movies", Rating: "PG"},
		{ID: "mature", Library: "Movies", Rating: "R"},
		{ID: "other", Library: "Shows", Rating: "PG"},
	}, err: warning, safe: true}
	viewer := Profile{Rating: "family", Libraries: []string{"Movies"}}
	items, err := VisibleLibrary(index, viewer)
	if len(items) != 1 || items[0].ID != "allowed" || !errors.Is(err, warning) {
		t.Fatalf("visible library = %#v, %v", items, err)
	}
	if item, found := VisibleItem(index, viewer, "allowed"); !found || item.ID != "allowed" {
		t.Fatalf("visible item = %#v, %t", item, found)
	}
	if _, found := VisibleItem(index, viewer, "missing"); found {
		t.Fatal("missing item was visible")
	}
	index.safe = false
	if _, found := VisibleItem(index, viewer, "allowed"); found {
		t.Fatal("unsafe item was visible")
	}
	index.safe = true
	if _, found := VisibleItem(index, viewer, "mature"); found {
		t.Fatal("policy-denied item was visible")
	}
}
