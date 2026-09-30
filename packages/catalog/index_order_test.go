package catalog_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestDecoratorPublicationPreservesPreviousReferences(t *testing.T) {
	index := catalog.NewMemoryIndex([]library.Item{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}, true)
	previous, err := index.References()
	if err != nil {
		t.Fatal(err)
	}
	index.SetDecorator(func(items []library.Item) []library.Item {
		items[0].Title, items[1].Title = "Zebra", "Apple"
		return items
	})
	current, err := index.References()
	if err != nil {
		t.Fatal(err)
	}
	if got := []string{previous[0].Title, previous[1].Title}; !reflect.DeepEqual(got, []string{"Alpha", "Beta"}) {
		t.Fatalf("decorator mutated the previously published read-only snapshot: %v", got)
	}
	if got := []string{current[0].Title, current[1].Title}; !reflect.DeepEqual(got, []string{"Zebra", "Apple"}) {
		t.Fatalf("decorator did not publish its updated titles: %v", got)
	}
}

// The index-owned path must preserve the existing public browse operation,
// including fields derived after selection rather than just returned item IDs.
func TestIndexedBrowsePreservesLocalePagesAndCurrentProfileState(t *testing.T) { //nolint:cyclop,gocognit,funlen // One matrix compares every selected result field across locales, views, and changing profile state.
	items := []library.Item{
		{ID: "z", Title: "Zebra", Kind: "video", Year: "2024"},
		{ID: "ae", Title: "Ängel", Kind: "video", Year: "2023"},
		{ID: "a", Title: "Alpha", Kind: "video", Year: "2022"},
		{ID: "aa", Title: "Åland", Kind: "video", Year: "2021"},
		{ID: "sort", Title: "Other", SortTitle: "!Alpha", Kind: "video"},
		{ID: "e2", Title: "Episode 2", Show: "show", ShowTitle: "Show", Kind: "video", Season: 1, Episode: 2},
		{ID: "e1", Title: "Episode 1", Show: "show", ShowPlot: "Plot", Kind: "video", Season: 1, Episode: 1},
		{ID: "song", Title: "Song", Kind: "audio"},
		{ID: "book", Title: "Élan", Kind: "book"},
		{ID: "spoken", Title: "Narration", Kind: "audiobook"},
		{ID: "photo", Title: "Sunset", Kind: "photo"},
	}
	index := catalog.NewMemoryIndex(items, true)
	progress, listed := map[string]catalog.PlaybackState{}, map[string]bool{}
	var pm sync.Mutex
	var lm sync.RWMutex
	profile, hidden := "guest", "a"
	listed["guest:z"] = true
	progress["guest:z"] = catalog.PlaybackState{Watched: true, Updated: time.Unix(100, 0)}
	access := func() catalog.BrowseAccess {
		return catalog.NewBrowseAccess(&pm, &lm, &progress, &listed, profile, profile == "owner", func(item library.Item) bool { return item.ID != hidden })
	}
	for pass := range 2 {
		if pass == 1 {
			profile, hidden = "owner", ""
		}
		for _, locale := range []string{"en", "sv", "fr", "tr", "ja", "en"} {
			for _, values := range []url.Values{
				// Native entry routes must work before an all-title order exists.
				{"view": {"movies"}},
				{"view": {"music"}},
				{"view": {"books"}},
				{"view": {"audiobooks"}},
				{"view": {"photos"}},
				nil,
				{"limit": {"2"}, "offset": {"2"}},
				{"letter": {"A"}, "limit": {"1"}},
				{"view": {"shows"}},
				{"view": {"list"}},
				{"view": {"unwatched"}},
				{"view": {"history"}},
				{"sort": {"added"}},
				{"sort": {"year"}},
				{"q": {"Episode"}},
			} {
				want, wantErr := catalog.BrowseLibrary(t.Context(), values, locale, index.References, access)
				// Read immediately again before other view keys can evict this order.
				for repetition := range 2 {
					got, gotErr := index.BrowseLibrary(t.Context(), values, locale, access)
					if !reflect.DeepEqual(gotErr, wantErr) || !reflect.DeepEqual(got.AllItems(), want.AllItems()) ||
						!reflect.DeepEqual(got.Items, want.Items) || !reflect.DeepEqual(got.Letters, want.Letters) ||
						got.Total != want.Total || got.Offset != want.Offset || got.NextURL() != want.NextURL() || got.PreviousURL() != want.PreviousURL() {
						t.Fatalf("pass=%d repetition=%d locale=%s values=%v got=%#v err=%v want=%#v err=%v", pass, repetition, locale, values, got, gotErr, want, wantErr)
					}
				}
			}
		}
	}
	// Repeated warm reads cannot cache current list or watched state.
	profile, hidden = "guest", "a"
	listed["guest:z"] = false
	result, err := index.BrowseLibrary(t.Context(), url.Values{"view": {"list"}}, "en", access)
	if err != nil || result.Total != 0 {
		t.Fatalf("stale list: %#v %v", result, err)
	}
	progress["guest:z"] = catalog.PlaybackState{}
	hidden = ""
	result, err = index.BrowseLibrary(t.Context(), url.Values{"view": {"unwatched"}}, "en", access)
	if err != nil || result.Total != len(items) {
		t.Fatalf("stale watched state: %#v %v", result, err)
	}
}

func TestIndexedBrowsePublicationAndRejectedRequests(t *testing.T) { //nolint:cyclop // One lifecycle test covers successful publication, failed scans, and rejected requests.
	index := catalog.NewMemoryIndex([]library.Item{{ID: "a", Title: "Alpha"}, {ID: "b", Title: "Beta"}}, true)
	access := orderTestAccess()
	read := func() []string {
		t.Helper()
		result, err := index.BrowseLibrary(t.Context(), nil, "en", access)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(result.Items))
		for i, item := range result.Items {
			ids[i] = item.ID
		}
		return ids
	}
	if got := read(); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatal(got)
	}
	index.SetDecorator(func(items []library.Item) []library.Item {
		items[0].Title, items[1].Title = "Zebra", "Apple"
		return items
	})
	if got := read(); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("stale decorator order: %v", got)
	}
	index.SetRoots([]catalog.ScanRoot{{Path: t.TempDir() + "/missing"}})
	if err := index.Refresh(t.Context()); err == nil {
		t.Fatal("missing root scan succeeded")
	}
	if got := read(); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Fatalf("failed scan discarded catalog: %v", got)
	}
	index.SetDecorator(func(items []library.Item) []library.Item { return items })
	index.SetRoots(nil)
	if err := index.Refresh(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := read(); len(got) != 0 {
		t.Fatalf("successful empty scan retained old items: %v", got)
	}
}

func TestIndexedBrowseRejectedRequestsSkipProfileState(t *testing.T) {
	index := catalog.NewMemoryIndex(nil, true)
	for _, values := range []url.Values{
		{"unknown": {"x"}},
		{"q": {"a", "b"}},
		{"q": {strings.Repeat("x", 513)}},
		{"q": {string([]byte{255})}},
		{"view": {"invalid"}},
		{"sort": {"invalid"}},
		{"offset": {"-1"}},
		{"limit": {"0"}},
		{"letter": {"A"}, "q": {"a"}},
	} {
		_, err := index.BrowseLibrary(t.Context(), values, "en", func() catalog.BrowseAccess {
			t.Fatal("rejected query read profile state")
			return catalog.BrowseAccess{}
		})
		if !errors.Is(err, catalog.ErrInvalidBrowse) {
			t.Fatalf("query=%v err=%v", values, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := index.BrowseLibrary(ctx, nil, "en", func() catalog.BrowseAccess {
		t.Fatal("canceled query read profile state")
		return catalog.BrowseAccess{}
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestIndexedBrowseConcurrentReadersAndPublication(t *testing.T) { //nolint:cyclop,gocognit // Concurrent publication and canceled readers share one observable result contract.
	items := make([]library.Item, 2048)
	for i := range items {
		items[i] = library.Item{ID: strconv.Itoa(i), Title: fmt.Sprintf("Title %04d", len(items)-i)}
	}
	index := catalog.NewMemoryIndex(items, true)
	var wg sync.WaitGroup
	for worker := range 12 {
		wg.Go(func() {
			access := orderTestAccess()
			for iteration := range 8 {
				ctx, cancel := context.WithCancel(t.Context())
				if iteration%3 == 0 {
					cancel()
				}
				locale := []string{"en", "sv", "fr", "tr", "ja", "de"}[worker%6]
				result, err := index.BrowseLibrary(ctx, nil, locale, access)
				cancel()
				if iteration%3 == 0 {
					if !errors.Is(err, context.Canceled) {
						t.Errorf("canceled reader: %v", err)
					}
				} else if err != nil || result.Total != len(items) || len(result.Items) != 100 {
					t.Errorf("concurrent result total=%d count=%d err=%v", result.Total, len(result.Items), err)
				}
			}
		})
	}
	for range 4 {
		index.SetDecorator(func(items []library.Item) []library.Item {
			for i := range items {
				items[i].Title = "!" + items[i].Title
			}
			return items
		})
	}
	wg.Wait()
}

func TestIndexedBrowseDuplicateIDTiesMatchUncachedSelection(t *testing.T) {
	index := catalog.NewMemoryIndex([]library.Item{
		{ID: "same", Title: "Alpha", Kind: "audio"},
		{ID: "same", Title: "Alpha", Kind: "video"},
		{ID: "unique", Title: "Alpha", Kind: "video"},
	}, true)
	for _, view := range []string{"all", "movies", "music"} {
		values := url.Values{"view": {view}}
		want, err := catalog.BrowseLibrary(t.Context(), values, "en", index.References, orderTestAccess())
		if err != nil {
			t.Fatal(err)
		}
		got, err := index.BrowseLibrary(t.Context(), values, "en", orderTestAccess())
		if err != nil || !reflect.DeepEqual(got.Items, want.Items) {
			t.Fatalf("view=%s got=%#v want=%#v err=%v", view, got.Items, want.Items, err)
		}
	}
}

func orderTestAccess() func() catalog.BrowseAccess {
	var pm sync.Mutex
	var lm sync.RWMutex
	progress, listed := map[string]catalog.PlaybackState{}, map[string]bool{}
	return func() catalog.BrowseAccess {
		return catalog.NewBrowseAccess(&pm, &lm, &progress, &listed, "owner", true, func(library.Item) bool { return true })
	}
}
