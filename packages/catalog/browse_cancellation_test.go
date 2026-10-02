package catalog

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestCancelledBrowseSkipsLoadAndAccess(t *testing.T) {
	for _, expired := range []bool{false, true} {
		ctx, cancel := context.WithCancel(t.Context())
		want := context.Canceled
		if expired {
			cancel()
			ctx, cancel = context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			want = context.DeadlineExceeded
		} else {
			cancel()
		}
		defer cancel()
		loads, accesses := 0, 0
		result, err := BrowseLibrary(ctx, nil, "en", func() ([]*library.Item, error) { loads++; return nil, nil }, func() BrowseAccess { accesses++; return BrowseAccess{} })
		if !errors.Is(err, want) || loads != 0 || accesses != 0 || result.Total != 0 || len(result.Items) != 0 {
			t.Fatalf("cancelled result=%#v error=%v loads=%d accesses=%d", result, err, loads, accesses)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	loads := 0
	_, err := BrowseLibrary(ctx, url.Values{"unknown": {"x"}}, "en", func() ([]*library.Item, error) { loads++; return nil, nil }, func() BrowseAccess { t.Fatal("invalid query accessed state"); return BrowseAccess{} })
	if !errors.Is(err, ErrInvalidBrowse) || loads != 0 {
		t.Fatalf("invalid cancelled query loaded state: %v", err)
	}
}

func TestBrowseCancellationAfterLoadSkipsAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	accesses := 0
	_, err := BrowseLibrary(ctx, nil, "en", func() ([]*library.Item, error) { cancel(); return []*library.Item{{ID: "one", Title: "One"}}, nil }, func() BrowseAccess { accesses++; return BrowseAccess{} })
	if !errors.Is(err, context.Canceled) || accesses != 0 {
		t.Fatalf("cancelled load error=%v accesses=%d", err, accesses)
	}
}

func TestBrowseCancellationDuringProjectionReleasesProfileLocks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	items := make([]*library.Item, 1024)
	for position := range items {
		items[position] = &library.Item{ID: strconv.Itoa(position), Title: "Visible"}
	}
	var progressMutex sync.Mutex
	var listMutex sync.RWMutex
	progress := map[string]PlaybackState{"viewer:0": {Watched: true}}
	listed := map[string]bool{"viewer:0": true}
	seen := 0
	result, err := BrowseLibrary(ctx, nil, "en", func() ([]*library.Item, error) { return items, nil }, func() BrowseAccess {
		return NewBrowseAccess(&progressMutex, &listMutex, &progress, &listed, "viewer", false, func(library.Item) bool {
			seen++
			if seen == 4 {
				cancel()
			}
			return true
		})
	})
	requireCancelledBrowseResult(t, result, err)
	if seen < 4 || seen >= len(items) {
		t.Fatalf("cancelled projection visited %d items", seen)
	}
	if !progressMutex.TryLock() {
		t.Fatal("cancelled projection retained progress lock")
	}
	progressMutex.Unlock()
	if !listMutex.TryLock() {
		t.Fatal("cancelled projection retained list lock")
	}
	listMutex.Unlock()
	if !reflect.DeepEqual(progress, map[string]PlaybackState{"viewer:0": {Watched: true}}) || !reflect.DeepEqual(listed, map[string]bool{"viewer:0": true}) || items[0].ID != "0" {
		t.Fatal("cancelled browse changed profile or item state")
	}
}

func TestCancelledApplyKeepsCallerCandidatesAndReturnsNoPage(t *testing.T) {
	browse, err := ParseBrowse(nil, "en")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	candidates := []Candidate{{Item: &library.Item{ID: "z", Title: "Zulu"}}, {Item: &library.Item{ID: "a", Title: "Alpha"}}}
	result, err := browse.Apply(ctx, candidates)
	requireCancelledBrowseResult(t, result, err)
	if candidates[0].Item.ID != "z" || candidates[1].Item.ID != "a" {
		t.Fatal("cancelled apply changed caller candidates")
	}
}

func requireCancelledBrowseResult(t *testing.T, result Result, err error) {
	t.Helper()
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, Result{}) {
		t.Fatalf("cancelled browse returned a partial result: %#v, %v", result, err)
	}
}
