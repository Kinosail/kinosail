package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

func browseParityItems(count int) []library.Item {
	items := snapshotFixtureItems(count)
	titles := []string{"Ångström", "Zebra Crossing", "émile", "Äpple", "100 Days", "The Matrix", "Øresund", "apple pie", "Çay", "Ölfest"}
	kinds := []string{"book", "photo", "audiobook"}
	for position := range items {
		if position%5 == 0 {
			items[position].Library = "Other"
		}
		if items[position].Show == "" && items[position].Album == "" && position%3 == 0 {
			items[position].Title = fmt.Sprintf("%s %04d", titles[position%len(titles)], position)
		}
		if position%37 == 0 && items[position].Show == "" {
			items[position].Kind = kinds[position%len(kinds)]
		}
		if position%11 == 0 {
			items[position].Rating = ""
		}
	}
	return items
}

func browseParityTargets() []string {
	targets := make([]string, 0, 64)
	for _, view := range []string{"", "all", "movies", "shows", "music", "audiobooks", "books", "photos", "collections", "playlists", "list", "unwatched", "history"} {
		for _, query := range []string{"", "&lang=sv", "&lang=de", "&offset=100&limit=50", "&sort=added", "&q=apple"} {
			targets = append(targets, "/api/v1/library?view="+view+query)
		}
	}
	return append(targets, "/api/v1/library?view=movies&letter=Z", "/api/v1/library?view=all&letter=%C3%85&lang=sv", "/api/v1/library?letter=M&offset=100", "/api/v1/library?view=shows&letter=S&limit=1", "/api/v1/library?view=movies&letter=Q")
}

func TestLibraryBrowseMatchesUncachedVisibilityForEveryPolicy(t *testing.T) {
	t.Parallel()
	index := memoryLibraryIndex(browseParityItems(1500), true)
	progress, lists := newProgressStore(""), newListStore("")
	profiles := append(append([]viewerProfile(nil), snapshotProfiles...), viewerProfile{ID: "kids", Rating: "family", Libraries: []string{"Other"}})
	published, _ := index.Snapshot()
	for position, profile := range profiles {
		for _, item := range published[position*30 : position*30+15] {
			if err := progress.Set(snapshotRequest(t.Context(), http.MethodPut, "/", profile), item.ID, float64(position+1), nil); err != nil {
				t.Fatal(err)
			}
			lists.values[profile.ID+":"+item.ID] = true
		}
	}
	cachedResult := func(request *http.Request) (catalog.Result, error) {
		return browseLibrary(request, index, progress, lists)
	}
	uncachedResult := func(request *http.Request) (catalog.Result, error) {
		snapshot, err := index.Library()
		if err != nil {
			return catalog.Result{}, err
		}
		return catalog.BrowseLibrary(request.Context(), request.URL.Query(), preferredLanguage(request), func() ([]*library.Item, error) {
			items := snapshot.Items()
			references := make([]*library.Item, len(items))
			for position := range items {
				references[position] = &items[position]
			}
			return references, nil
		}, func() catalog.BrowseAccess {
			return progress.BrowseAccess(&lists.mu, &lists.values, request, viewerPolicy(currentViewer(request)).Allows)
		})
	}
	cached, uncached := catalogapi.Library(cachedResult, progress.ClientItem), catalogapi.Library(uncachedResult, progress.ClientItem)
	compare := func(t *testing.T, phase string) {
		t.Helper()
		var requests sync.WaitGroup
		for _, profile := range profiles {
			for _, target := range browseParityTargets() {
				requests.Go(func() {
					want, got := httptest.NewRecorder(), httptest.NewRecorder()
					uncached(want, snapshotRequest(t.Context(), http.MethodGet, target, profile))
					cached(got, snapshotRequest(t.Context(), http.MethodGet, target, profile))
					if got.Code != want.Code || got.Body.String() != want.Body.String() {
						t.Errorf("%s %s %s = %d %.300q, want %d %.300q", phase, profile.ID, target, got.Code, got.Body.String(), want.Code, want.Body.String())
					}
					wantPage, wantErr := uncachedResult(snapshotRequest(t.Context(), http.MethodGet, target, profile))
					gotPage, gotErr := cachedResult(snapshotRequest(t.Context(), http.MethodGet, target, profile))
					if (gotErr == nil) != (wantErr == nil) || !reflect.DeepEqual(gotPage.Letters, wantPage.Letters) || gotPage.PreviousURL() != wantPage.PreviousURL() || gotPage.NextURL() != wantPage.NextURL() {
						t.Errorf("%s %s %s web links = %+v %q %q, want %+v %q %q", phase, profile.ID, target, gotPage.Letters, gotPage.PreviousURL(), gotPage.NextURL(), wantPage.Letters, wantPage.PreviousURL(), wantPage.NextURL())
					}
				})
			}
		}
		requests.Wait()
	}
	compare(t, "cold")
	compare(t, "warm")
	index.SetDecorator(func(items []library.Item) []library.Item {
		for position := range items {
			if position%4 == 0 {
				items[position].Rating, items[position].Title = "R", "Yellow "+items[position].Title
			}
		}
		return items
	})
	compare(t, "republished")
	if t.Failed() {
		return
	}
	snapshot, _ := index.Library()
	for _, profile := range profiles {
		response := httptest.NewRecorder()
		cached(response, snapshotRequest(t.Context(), http.MethodGet, "/api/v1/library?limit=200&"+url.Values{"view": {"all"}}.Encode(), profile))
		for _, item := range snapshot.Items() {
			if !viewerPolicy(profile).Allows(item) && strings.Contains(response.Body.String(), `"id":"`+item.ID+`"`) {
				t.Fatalf("%s sees hidden item %s", profile.ID, item.ID)
			}
		}
	}
}
