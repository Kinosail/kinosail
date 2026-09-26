package home

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestProjectionHelpers(t *testing.T) { //nolint:cyclop // The helper contract covers each header and projection boundary.
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "Movie"},
		{ID: "episode-art", Kind: "video", Title: "One", Show: "series", ShowTitle: "Series", ShowArtwork: "poster", Season: 1, Episode: 1},
		{ID: "episode-fallback", Kind: "video", Title: "Two", Show: "series", ShowTitle: "Series", Season: 1, Episode: 2},
	}
	artwork, titles := MediaPresentation(items, func(item library.Item) bool { return item.ID != "episode-fallback" })
	if artwork["movie"] != "movie" || artwork["episode-art"] != "episode-art" || artwork["episode-fallback"] != "episode-art" || titles["episode-fallback"] != "Series" {
		t.Fatalf("artwork=%v titles=%v", artwork, titles)
	}
	if matched := Media(items, "video"); len(matched) != 3 || len(Media(items, "book")) != 0 {
		t.Fatalf("matched=%v", matched)
	}

	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test/?q=movie&offset=10&letter=M&view=movies", nil)
	if clear := ClearSearchURL(request); clear != "/?view=movies" {
		t.Fatalf("clear URL = %q", clear)
	}
	if clear := ClearSearchURL(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test/?q=movie", nil)); clear != "/" {
		t.Fatalf("empty clear URL = %q", clear)
	}
	if fragment, err := FragmentRequest(request); err != nil || fragment {
		t.Fatalf("empty fragment = %v, %v", fragment, err)
	}
	request.Header.Set(InfiniteLibraryHeader, "1")
	if fragment, err := FragmentRequest(request); err != nil || !fragment {
		t.Fatalf("valid fragment = %v, %v", fragment, err)
	}
	request.Header.Set(InfiniteLibraryHeader, "0")
	if _, err := FragmentRequest(request); err == nil {
		t.Fatal("invalid fragment was accepted")
	}
	request.Header.Add(InfiniteLibraryHeader, "1")
	if _, err := FragmentRequest(request); err == nil {
		t.Fatal("repeated fragment was accepted")
	}
}

func TestDestinationProjection(t *testing.T) {
	items := []library.Item{
		{ID: "movie", Kind: "video", Title: "Movie"},
		{ID: "episode", Kind: "video", Title: "Episode", Show: "series"},
		{ID: "song", Kind: "audio", Title: "Song"},
		{ID: "spoken", Kind: "audiobook", Title: "Spoken"},
		{ID: "book", Kind: "book", Title: "Book"},
		{ID: "photo", Kind: "photo", Title: "Photo"},
	}
	groups := Destinations(items, 1, 2, 3)
	if len(groups) != 3 || len(groups[0].Destinations) != 3 || len(groups[1].Destinations) != 3 || len(groups[2].Destinations) != 3 {
		t.Fatalf("groups=%+v", groups)
	}
	if groups[0].Destinations[0].CountLabel != "1 item" || groups[2].Destinations[1].CountLabel != "2 items" || CountLabel(0) != "0 items" {
		t.Fatalf("count labels=%+v", groups)
	}
	if groups := Destinations(nil, 0, 0, 0); len(groups) != 0 {
		t.Fatalf("empty groups=%+v", groups)
	}
}

func TestRecentShelfProjection(t *testing.T) { //nolint:cyclop // One fixture verifies the complete recent-shelf projection contract.
	now := time.Now()
	items := []library.Item{
		{ID: "episode-one", Kind: "video", Title: "Pilot", Show: "series", ShowTitle: "Series", Season: 1, Episode: 1, Container: "MKV", Year: "2025", Added: now},
		{ID: "episode-two", Kind: "video", Title: "Second", Show: "series", ShowTitle: "Series", Season: 1, Episode: 2, Container: "MP4", Added: now.Add(-time.Minute)},
		{ID: "book", Kind: "book", Title: "Book", Year: "2024", Added: now.Add(-2 * time.Minute)},
		{ID: "movie", Kind: "video", Title: "Movie", Container: "AVI", Added: now.Add(-3 * time.Minute)},
	}
	artwork := map[string]string{"episode-one": "poster", "book": "cover"}
	titles := map[string]string{"episode-one": "Series"}
	played := RecentlyPlayed(items, artwork, titles)
	if len(played) != 3 || !played[0].Stacked || played[0].Count != 2 || played[0].Title != "Series" || played[0].StackLabel != "recently played episodes stacked" || played[0].Meta != "2 episodes stacked · MP4" {
		t.Fatalf("played=%+v", played)
	}
	if played[1].Href != "/book/book" || played[1].Meta != "2024" || played[2].Href != "/watch/movie" || played[2].Meta != "AVI" || !played[0].Priority || !played[1].Priority || played[2].Priority {
		t.Fatalf("played details=%+v", played)
	}
	addedItems := make([]library.Item, 14)
	for index := range addedItems {
		addedItems[index] = library.Item{ID: string(rune('a' + index)), Kind: "book", Title: "Book", Added: now.Add(time.Duration(index) * time.Minute)}
	}
	added := RecentlyAdded(addedItems, nil, nil)
	if len(added) != recentItemLimit || added[0].Href != "/book/n" || added[0].PlaceholderIcon != "plus" {
		t.Fatalf("added=%+v", added)
	}
	addedItems[12].Added = addedItems[13].Added
	added = RecentlyAdded(addedItems, nil, nil)
	if added[0].Href != "/book/m" || added[1].Href != "/book/n" || addedItems[0].ID != "a" || addedItems[13].ID != "n" {
		t.Fatalf("added order=%+v, input first=%q last=%q", added[:2], addedItems[0].ID, addedItems[13].ID)
	}
	if item := singleItem(items[0], artwork, titles, "play"); item.Title != "Series · Pilot" || item.Meta != "MKV · 2025" || item.ArtworkID != "poster" || item.Count != 1 {
		t.Fatalf("single=%+v", item)
	}
	if meta := stackMeta(2, library.Item{}); meta != "2 episodes stacked" {
		t.Fatalf("stack meta=%q", meta)
	}
}
