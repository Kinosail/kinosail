package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

var subtitleBrowseProfiles = []viewerProfile{
	{ID: "owner", Owner: true},
	{ID: "teen", Rating: "teen", Libraries: []string{"all"}},
	{ID: "family", Rating: "family", Libraries: []string{"all"}},
	{ID: "default", Libraries: []string{"Library"}},
	{ID: "elsewhere", Rating: "all", Libraries: []string{"Other"}},
}

func subtitleBrowseItems(count int) []library.Item {
	items := make([]library.Item, 0, count)
	titles := []string{"Ångström", "Zebra", "émile", "Äpple", "100 Days", "The Matrix", "Øresund", "apple", "Çay", "Movie"}
	ratings := []string{"PG", "PG-13", "R", ""}
	added := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for position := range count {
		item := library.Item{ID: fmt.Sprintf("%016x", position), Library: []string{"Library", "Other"}[position%5/4], Kind: "video", Rating: ratings[position%len(ratings)], Container: "MP4", Year: "2026", Added: added.Add(time.Duration(position) * time.Minute), Title: fmt.Sprintf("%s %04d", titles[position%len(titles)], position)}
		switch position % 9 {
		case 1, 2, 3:
			item.Show, item.Season, item.Episode = fmt.Sprintf("Show %03d", position/30), 1, position%30
		case 4:
			item.Kind, item.Album, item.Artist = "audio", fmt.Sprintf("Album %03d", position/40), "Artist"
		case 5:
			item.Kind = []string{"book", "photo", "audiobook"}[position%3]
		}
		items = append(items, item)
	}
	return items
}

func subtitleBrowseRequest(ctx context.Context, target string, profile viewerProfile) *http.Request {
	return httptest.NewRequestWithContext(context.WithValue(ctx, viewerContextKey{}, profile), http.MethodGet, target, nil)
}

func TestSubtitlesLibraryBrowseMatchesUncachedVisibilityForEveryPolicy(t *testing.T) {
	t.Parallel()
	index := memoryLibraryIndex(subtitleBrowseItems(1200), true)
	cached := catalogapi.Library(func(request *http.Request) (catalog.Result, error) { return browseLibrary(request, index) }, libraryClientItem)
	uncached := catalogapi.Library(func(request *http.Request) (catalog.Result, error) {
		snapshot, err := index.Library()
		if err != nil {
			return catalog.Result{}, err
		}
		return catalog.NewMemoryIndex(snapshot.Items(), true).BrowseLibrary(request.Context(), request.URL.Query(), preferredLanguage(request), func() catalog.BrowseAccess {
			viewer := currentViewer(request)
			return catalog.NewBrowseAccess(&browseProgressMu, &browseListMu, &browseProgress, &browseListed, viewer.ID, viewer.Owner, viewerPolicy(viewer).Allows)
		})
	}, libraryClientItem)
	targets := []string{"/api/v1/library?view=movies&letter=Z", "/api/v1/library?view=shows&letter=S&limit=1", "/api/v1/library?letter=%C3%85&lang=sv"}
	for _, view := range []string{"", "all", "movies", "shows", "music", "audiobooks", "books", "photos", "collections", "list", "unwatched", "history"} {
		for _, query := range []string{"", "&lang=sv", "&lang=de", "&offset=100&limit=50", "&sort=added", "&q=apple"} {
			targets = append(targets, "/api/v1/library?view="+view+query)
		}
	}
	compare := func(phase string) {
		var requests sync.WaitGroup
		for _, profile := range subtitleBrowseProfiles {
			for _, target := range targets {
				requests.Go(func() {
					want, got := httptest.NewRecorder(), httptest.NewRecorder()
					uncached(want, subtitleBrowseRequest(t.Context(), target, profile))
					cached(got, subtitleBrowseRequest(t.Context(), target, profile))
					if got.Code != want.Code || got.Body.String() != want.Body.String() {
						t.Errorf("%s %s %s = %d %.300q, want %d %.300q", phase, profile.ID, target, got.Code, got.Body.String(), want.Code, want.Body.String())
					}
				})
			}
		}
		requests.Wait()
	}
	compare("cold")
	compare("warm")
	index.SetDecorator(func(items []library.Item) []library.Item {
		for position := range items {
			if position%4 == 0 {
				items[position].Rating, items[position].Title = "R", "Yellow "+items[position].Title
			}
		}
		return items
	})
	compare("republished")
}

func BenchmarkSubtitlesLibraryBrowse(b *testing.B) {
	index := memoryLibraryIndex(subtitleBrowseItems(20_000), true)
	handler := catalogapi.Library(func(request *http.Request) (catalog.Result, error) { return browseLibrary(request, index) }, libraryClientItem)
	for _, profile := range []viewerProfile{subtitleBrowseProfiles[0], subtitleBrowseProfiles[1]} {
		b.Run(profile.ID, func(b *testing.B) {
			request := subtitleBrowseRequest(b.Context(), "/api/v1/library?view=movies&offset=100", profile)
			b.ReportAllocs()
			for b.Loop() {
				response := httptest.NewRecorder()
				handler(response, request)
				if response.Code != http.StatusOK {
					b.Fatal(response.Code)
				}
			}
		})
	}
}
