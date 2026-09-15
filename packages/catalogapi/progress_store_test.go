package catalogapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

func TestProgressStoreUsesCanonicalViewerProjections(t *testing.T) {
	profile := identitycore.Profile{ID: "viewer", Name: "Viewer", Owner: true}
	store := NewProgressStore("", nil, nil, func(string, any) error { return nil }, func(*http.Request) identitycore.Profile { return profile })
	store.Replace(map[string]catalog.PlaybackState{"viewer:movie": {Seconds: 42, Updated: time.Now()}})
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	item := library.Item{ID: "movie", Kind: "video", Title: "Movie"}
	projected := store.ClientItem(request, item).(ClientItem)
	if projected.ID != "movie" || projected.Progress.Seconds != 42 || projected.Stream != "/media/movie" || projected.Download != "/download/movie" {
		t.Fatalf("projected item = %#v", projected)
	}
	index := catalog.NewMemoryIndex([]library.Item{item}, true)
	activity := RecentAdminProgress(store, index, []identitycore.Profile{profile})
	if len(activity) != 1 || activity[0].Profile != "Viewer" || activity[0].Title != "Movie" {
		t.Fatalf("recent activity = %#v", activity)
	}
}
