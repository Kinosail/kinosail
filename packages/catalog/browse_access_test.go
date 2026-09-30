package catalog_test

import (
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestBrowseAccessPreservesCallerReferencesAndFreshViewState(t *testing.T) { //nolint:cyclop,funlen // One owner-boundary matrix checks literal view membership and caller storage ownership.
	profile := strings.Repeat("a", 26)
	refs := []*library.Item{
		{ID: "z", Title: "Zulu", Kind: "video", Added: time.Unix(20, 0)}, nil,
		{ID: "a", Title: "Alpha", Kind: "video", Added: time.Unix(10, 0)},
		{ID: "e2", Title: "Episode Beta", Kind: "video", Show: "Series", ShowTitle: "Series", ShowPlot: "Merged plot"},
		{ID: "e1", Title: "Episode Alpha", Kind: "video", Show: "Series", ShowTitle: "Series"},
		{ID: "song", Title: "Beta Song", Kind: "audio"},
		{ID: "book", Title: "Café Book", Kind: "book"},
		{ID: "spoken", Title: "Delta", Kind: "audiobook"},
		{ID: "photo", Title: "Echo", Kind: "photo"},
		{ID: "hidden", Title: "Hidden", Kind: "video"},
	}
	before := append([]*library.Item(nil), refs...)
	progress := map[string]catalog.PlaybackState{
		profile + ":a": {Watched: true, Updated: time.Unix(100, 0)},
		profile + ":z": {},
		"z":            {Watched: true, Updated: time.Unix(200, 0)},
	}
	listed := map[string]bool{profile + ":a": true, profile + ":hidden": true}
	var pm sync.Mutex
	var lm sync.RWMutex
	read := func(values url.Values, owner bool) catalog.Result {
		t.Helper()
		browse, err := catalog.ParseBrowse(values, "en")
		if err != nil {
			t.Fatal(err)
		}
		result, err := browse.ApplyAccess(t.Context(), refs, catalog.NewBrowseAccess(&pm, &lm, &progress, &listed, profile, owner, func(item library.Item) bool { return item.ID != "hidden" }))
		if err != nil || !reflect.DeepEqual(refs, before) {
			t.Fatalf("browse modified caller references or failed: %v", err)
		}
		return result
	}
	for _, test := range []struct {
		values url.Values
		ids    []string
	}{
		{nil, []string{"a", "song", "book", "spoken", "photo", "e1", "e2", "z"}},
		{url.Values{"view": {"movies"}}, []string{"a", "z"}},
		{url.Values{"view": {"movies"}, "sort": {"added"}}, []string{"z", "a"}},
		{url.Values{"view": {"shows"}}, []string{"e1"}},
		{url.Values{"view": {"music"}}, []string{"song"}},
		{url.Values{"view": {"books"}}, []string{"book"}},
		{url.Values{"view": {"audiobooks"}}, []string{"spoken"}},
		{url.Values{"view": {"photos"}}, []string{"photo"}},
		{url.Values{"view": {"list"}}, []string{"a"}},
		{url.Values{"view": {"history"}}, []string{"a"}},
		{url.Values{"q": {"CAFÉ"}}, []string{"book"}},
		{url.Values{"letter": {"A"}, "limit": {"1"}}, []string{"a"}},
	} {
		result := read(test.values, true)
		ids := make([]string, len(result.Items))
		for position, item := range result.Items {
			ids[position] = item.ID
		}
		if !reflect.DeepEqual(ids, test.ids) {
			t.Fatalf("values=%v IDs=%v want=%v", test.values, ids, test.ids)
		}
		if result.View == "shows" && result.Items[0].ShowPlot != "Merged plot" {
			t.Fatal("show projection lost visible episode metadata")
		}
	}
	if result := read(url.Values{"view": {"unwatched"}}, true); result.Total != 7 {
		t.Fatalf("present profile state did not override legacy watched state: %d", result.Total)
	}
	delete(progress, profile+":z")
	if result := read(url.Values{"view": {"unwatched"}}, true); result.Total != 6 {
		t.Fatalf("Owner did not use legacy watched state: %d", result.Total)
	}
	if result := read(url.Values{"view": {"unwatched"}}, false); result.Total != 7 {
		t.Fatalf("non-Owner inherited legacy watched state: %d", result.Total)
	}
	progress[profile+":a"] = catalog.PlaybackState{}
	listed[profile+":a"] = false
	if result := read(url.Values{"view": {"list"}}, true); result.Total != 0 {
		t.Fatalf("list mutation stayed stale: %d", result.Total)
	}
	if result := read(url.Values{"view": {"history"}}, true); result.Total != 1 || result.Items[0].ID != "z" {
		t.Fatalf("history mutation/fallback = %#v", result.Items)
	}
}
