package metadata

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	bulkIDOne = "0123456789abcdef"
	bulkIDTwo = "fedcba9876543210"
)

type bulkIndexStub struct {
	items       map[string]library.Item
	snapshot    []library.Item
	snapshotErr error
	refreshErr  error
	finds       int
	refreshes   int
	snapshots   int
	events      *[]string
}

func (index *bulkIndexStub) Find(id string) (library.Item, bool) {
	index.finds++
	if index.events != nil {
		*index.events = append(*index.events, "find:"+id)
	}
	item, found := index.items[id]
	return item, found
}

func (index *bulkIndexStub) Refresh(context.Context) error {
	index.refreshes++
	if index.events != nil {
		*index.events = append(*index.events, "refresh")
	}
	return index.refreshErr
}

func (index *bulkIndexStub) Snapshot() ([]library.Item, error) {
	index.snapshots++
	return append([]library.Item(nil), index.snapshot...), index.snapshotErr
}

type bulkEffects struct {
	records int
	saves   int
	events  []string
	saved   map[string]Record
	saveErr error
	initial map[string]Record
}

type bulkLock struct{ effects *bulkEffects }

func (lock bulkLock) Lock() {
	lock.effects.records++
	lock.effects.events = append(lock.effects.events, "records")
}

func (bulkLock) Unlock() {}

func (effects *bulkEffects) editor(index *bulkIndexStub) *BulkEditor {
	index.events = &effects.events
	return NewBulkEditor(index, bulkLock{effects}, &effects.initial, func(records map[string]Record) error {
		effects.saves++
		effects.events = append(effects.events, "save")
		effects.saved = records
		return effects.saveErr
	}, nil, nil)
}

func TestBulkEditorApplyPlayerSemantics(t *testing.T) {
	t.Parallel()
	items := map[string]library.Item{
		bulkIDOne: {ID: bulkIDOne, Title: "One", Year: "2001", Plot: "Plot", Rating: "PG", Tagline: "Tag", Genres: "Drama", Collection: "Set"},
		bulkIDTwo: {ID: bulkIDTwo, Title: "Two", Year: "2002", Plot: "Plot 2", Rating: "G", Tagline: "Tag 2", Genres: "Comedy", Collection: "Set 2"},
	}
	index := &bulkIndexStub{items: items}
	effects := &bulkEffects{initial: map[string]Record{}}
	values := []string{" New ", "2024", "New plot", "R", "New tag", "Action"}
	patch := BulkPatch{Title: &values[0], Year: &values[1], Plot: &values[2], Rating: &values[3], Tagline: &values[4], Genres: &values[5]}
	ids := []string{bulkIDOne, bulkIDTwo}
	updates, err := effects.editor(index).Apply(context.Background(), ids, patch)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !reflect.DeepEqual(ids, []string{bulkIDOne, bulkIDTwo}) {
		t.Fatalf("Apply() mutated ids = %#v", ids)
	}
	wantEvents := []string{"find:" + bulkIDOne, "find:" + bulkIDTwo, "records", "save", "refresh"}
	if !reflect.DeepEqual(effects.events, wantEvents) {
		t.Fatalf("side-effect order = %#v, want %#v", effects.events, wantEvents)
	}
	for _, id := range ids {
		got := updates[id]
		want := Record{Title: "New", Year: "2024", Plot: "New plot", Rating: "R", Tagline: "New tag", Genres: "Action", Collection: items[id].Collection, Owner: true}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("update %s = %#v, want %#v", id, got, want)
		}
	}
	if !reflect.DeepEqual(effects.saved, updates) {
		t.Fatalf("saved = %#v, want %#v", effects.saved, updates)
	}
}

func TestBulkEditorApplyPreservesStoredValues(t *testing.T) {
	t.Parallel()
	item := library.Item{ID: bulkIDOne, Title: "Scanned", Year: "1999", Plot: "Scanned plot", Rating: "G", Tagline: "Scanned tag", Genres: "Drama", Collection: "Scanned set"}
	stored := Record{Title: "Stored", Year: "2000", Plot: "Stored plot", Rating: "PG", Tagline: "Stored tag", Genres: "Comedy", Collection: "Stored set"}
	index := &bulkIndexStub{items: map[string]library.Item{bulkIDOne: item}}
	effects := &bulkEffects{initial: map[string]Record{bulkIDOne: stored}}
	rating := "R"
	updates, err := effects.editor(index).Apply(context.Background(), []string{bulkIDOne}, BulkPatch{Rating: &rating})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	stored.Rating, stored.Owner = rating, true
	if !reflect.DeepEqual(updates[bulkIDOne], stored) {
		t.Fatalf("update = %#v, want %#v", updates[bulkIDOne], stored)
	}
}

func TestBulkEditorRejectsInvalidInputBeforeEffects(t *testing.T) {
	t.Parallel()
	validTitle, badYear, alphaYear := "Title", "123", "20ab"
	blankTitle, oversizedTitle, controlRating := " \t ", strings.Repeat("x", 201), "PG\x00"
	overMaximum := make([]string, maxBulkItems+1)
	for position := range overMaximum {
		overMaximum[position] = bulkIDOne
	}
	tests := []struct {
		name  string
		ids   []string
		patch BulkPatch
		want  error
	}{
		{"missing selection", nil, BulkPatch{Title: &validTitle}, ErrBulkSelection},
		{"too many", overMaximum, BulkPatch{Title: &validTitle}, ErrBulkSelection},
		{"space", []string{" " + bulkIDOne}, BulkPatch{Title: &validTitle}, ErrBulkSelection},
		{"short", []string{"abc"}, BulkPatch{Title: &validTitle}, ErrBulkSelection},
		{"uppercase", []string{"0123456789abcdeF"}, BulkPatch{Title: &validTitle}, ErrBulkSelection},
		{"duplicate", []string{bulkIDOne, bulkIDOne}, BulkPatch{Title: &validTitle}, ErrBulkSelection},
		{"empty patch", []string{bulkIDOne}, BulkPatch{}, ErrBulkFields},
		{"short year", []string{bulkIDOne}, BulkPatch{Year: &badYear}, ErrBulkFields},
		{"nonnumeric year", []string{bulkIDOne}, BulkPatch{Year: &alphaYear}, ErrBulkFields},
		{"blank title", []string{bulkIDOne}, BulkPatch{Title: &blankTitle}, ErrBulkFields},
		{"oversized title", []string{bulkIDOne}, BulkPatch{Title: &oversizedTitle}, ErrBulkFields},
		{"control rating", []string{bulkIDOne}, BulkPatch{Rating: &controlRating}, ErrBulkFields},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			index := &bulkIndexStub{items: map[string]library.Item{}}
			effects := &bulkEffects{initial: map[string]Record{}}
			if _, err := effects.editor(index).Apply(context.Background(), test.ids, test.patch); !errors.Is(err, test.want) {
				t.Fatalf("Apply() error = %v, want %v", err, test.want)
			}
			if index.finds != 0 || effects.records != 0 || effects.saves != 0 || index.refreshes != 0 {
				t.Fatalf("invalid input effects = finds:%d records:%d saves:%d refreshes:%d", index.finds, effects.records, effects.saves, index.refreshes)
			}
		})
	}
}

func TestBulkEditorApplyFailures(t *testing.T) {
	t.Parallel()
	title, rating := "Title", "PG"
	tests := []struct {
		name       string
		item       library.Item
		patch      BulkPatch
		saveErr    error
		refreshErr error
		want       error
		refreshes  int
	}{
		{"missing item", library.Item{}, BulkPatch{Title: &title}, nil, nil, ErrBulkSelection, 0},
		{"invalid fallback", library.Item{ID: bulkIDOne}, BulkPatch{Rating: &rating}, nil, nil, ErrBulkFields, 0},
		{"save", library.Item{ID: bulkIDOne, Title: "Base"}, BulkPatch{Title: &title}, errors.New("disk"), nil, errBulkSave, 0},
		{"refresh", library.Item{ID: bulkIDOne, Title: "Base"}, BulkPatch{Title: &title}, nil, errors.New("scan"), errBulkSave, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items := map[string]library.Item{}
			if test.item.ID != "" {
				items[test.item.ID] = test.item
			}
			index := &bulkIndexStub{items: items, refreshErr: test.refreshErr}
			effects := &bulkEffects{initial: map[string]Record{}, saveErr: test.saveErr}
			if _, err := effects.editor(index).Apply(context.Background(), []string{bulkIDOne}, test.patch); !errors.Is(err, test.want) {
				t.Fatalf("Apply() error = %v, want %v", err, test.want)
			}
			if index.refreshes != test.refreshes {
				t.Fatalf("refreshes = %d, want %d", index.refreshes, test.refreshes)
			}
		})
	}
}
