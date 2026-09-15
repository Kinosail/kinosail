package library

import (
	"errors"
	"testing"
)

type visibilityStub struct {
	items []Item
	err   error
	safe  bool
}

func (stub visibilityStub) Snapshot() ([]Item, error) {
	return append([]Item(nil), stub.items...), stub.err
}

func (stub visibilityStub) Find(id string) (Item, bool) {
	for _, item := range stub.items {
		if item.ID == id {
			return item, true
		}
	}
	return Item{}, false
}
func (stub visibilityStub) Safe(string) bool { return stub.safe }

func TestVisibilityIndexFiltering(t *testing.T) {
	t.Parallel()
	index := visibilityStub{items: []Item{{ID: "allowed", Path: "/media/allowed"}, {ID: "denied", Path: "/media/denied"}}, err: errors.New("snapshot warning"), safe: true}
	items, err := Visible(index, func(item Item) bool { return item.ID == "allowed" })
	if len(items) != 1 || items[0].ID != "allowed" || !errors.Is(err, index.err) {
		t.Fatalf("visible = %#v, %v", items, err)
	}
	if item, found := VisibleItem(index, "allowed", func(Item) bool { return true }); !found || item.ID != "allowed" {
		t.Fatalf("visible item = %#v, %t", item, found)
	}
	if _, found := VisibleItem(index, "missing", func(Item) bool { return true }); found {
		t.Fatal("missing item was visible")
	}
	index.safe = false
	if _, found := VisibleItem(index, "allowed", func(Item) bool { return true }); found {
		t.Fatal("unsafe item was visible")
	}
	index.safe = true
	if _, found := VisibleItem(index, "allowed", func(Item) bool { return false }); found {
		t.Fatal("policy-denied item was visible")
	}
}

func TestViewerMaturityAndLibraryPolicy(t *testing.T) {
	t.Parallel()
	item := Item{Library: "movies", Rating: "PG-13"}
	for _, test := range []struct {
		name, rating string
		owner        bool
		libraries    []string
		item         Item
		want         bool
	}{
		{"library denied", "all", false, []string{"shows"}, item, false},
		{"owner", "", true, nil, item, true},
		{"all ratings", "all", false, []string{"movies"}, item, true},
		{"all libraries", "teen", false, []string{"all"}, item, true},
		{"family denied", "family", false, []string{"movies"}, item, false},
		{"default allowed", "", false, []string{"movies"}, Item{Library: "movies", Rating: "pg"}, true},
		{"unrated denied", "teen", false, []string{"movies"}, Item{Library: "movies", Rating: "NR"}, false},
	} {
		policy := NewPolicy(test.owner, test.rating, test.libraries)
		if got := policy.Allows(test.item); got != test.want {
			t.Errorf("%s Allows = %t, want %t", test.name, got, test.want)
		}
	}
}
