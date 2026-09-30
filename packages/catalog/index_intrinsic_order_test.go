package catalog_test

import (
	"fmt"
	"net/url"
	"reflect"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestIndexedIntrinsicBrowseRejectsLatePublicationOrder(t *testing.T) { //nolint:cyclop // One public lifecycle contract checks old in-flight data and repeated fresh results.
	index := catalog.NewMemoryIndex([]library.Item{
		{ID: "a", Title: "Alpha", Kind: "video"},
		{ID: "b", Title: "Beta", Kind: "video"},
		{ID: "song", Title: "Song", Kind: "audio"},
	}, true)
	var pm sync.Mutex
	var lm sync.RWMutex
	progress, listed := map[string]catalog.PlaybackState{}, map[string]bool{}
	published := false
	access := func() catalog.BrowseAccess {
		return catalog.NewBrowseAccess(&pm, &lm, &progress, &listed, "owner", true, func(library.Item) bool {
			if !published {
				published = true
				index.SetDecorator(func(items []library.Item) []library.Item {
					items[0].Title, items[1].Title = "Zebra", "Apple"
					return items
				})
			}
			return true
		})
	}
	values := url.Values{"view": {"movies"}}
	first, err := index.BrowseLibrary(t.Context(), values, "en", access)
	if err != nil || first.Total != 2 || first.Items[0].ID != "a" || first.Items[1].ID != "b" {
		t.Fatalf("in-flight snapshot: %#v %v", first.Items, err)
	}
	for range 3 {
		current, err := index.BrowseLibrary(t.Context(), values, "en", access)
		if err != nil || current.Total != 2 || current.Items[0].ID != "b" || current.Items[1].ID != "a" || current.Items[0].Title != "Apple" {
			t.Fatalf("late old request replaced the published movie order: %#v %v", current.Items, err)
		}
	}
}

func TestIndexedIntrinsicBrowsePreservesVisibilityAfterPartialFirstRead(t *testing.T) {
	for _, owner := range []bool{true, false} {
		t.Run(fmt.Sprintf("owner=%t", owner), func(t *testing.T) {
			index := catalog.NewMemoryIndex([]library.Item{
				{ID: "a", Title: "Alpha", Kind: "video"},
				{ID: "b", Title: "Beta", Kind: "video"},
				{ID: "song", Title: "Song", Kind: "audio"},
			}, true)
			var pm sync.Mutex
			var lm sync.RWMutex
			progress, listed := map[string]catalog.PlaybackState{}, map[string]bool{}
			hidden := "a"
			access := func() catalog.BrowseAccess {
				return catalog.NewBrowseAccess(&pm, &lm, &progress, &listed, "viewer", owner, func(item library.Item) bool { return item.ID != hidden })
			}
			for _, step := range []struct {
				hidden string
				ids    []string
			}{
				{"a", []string{"b"}},
				{"", []string{"a", "b"}},
				{"b", []string{"a"}},
				{"", []string{"a", "b"}},
			} {
				hidden = step.hidden
				result, err := index.BrowseLibrary(t.Context(), url.Values{"view": {"movies"}}, "en", access)
				ids := make([]string, len(result.Items))
				for i, item := range result.Items {
					ids[i] = item.ID
				}
				if err != nil || result.Total != len(step.ids) || !reflect.DeepEqual(ids, step.ids) {
					t.Fatalf("visibility=%q ids=%v total=%d err=%v want=%v", hidden, ids, result.Total, err, step.ids)
				}
			}
		})
	}
}
