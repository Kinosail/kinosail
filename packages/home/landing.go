package home

import (
	"slices"

	"github.com/MikeO7/kinosail/packages/library"
)

type landing struct {
	artwork, titles map[string]string
	recent          []ShelfItem
	genres          []MovieGenre
	shelves         map[string][]ShelfItem
	added           map[string][]*library.Item
	counts          libraryCounts
}

type libraryCounts struct {
	movies, shows, photos, music, audiobooks, books int
}

func libraryLanding(view *library.View, hasArtwork func(library.Item) bool) *landing {
	return library.Derive(view, "home.landing", func(view *library.View) *landing {
		items := view.Items()
		artwork, titles := MediaPresentation(items, hasArtwork)
		result := &landing{artwork: artwork, titles: titles, recent: RecentlyAdded(items, artwork, titles), genres: MovieGenres(items, artwork), shelves: make(map[string][]ShelfItem), added: make(map[string][]*library.Item), counts: countLibrary(view)}
		for _, kind := range []string{"movie", "show", "audio", "audiobook", "other"} {
			result.added[kind] = newestFirst(items, kind)
			result.shelves[kind] = recentShelf(result.added[kind], artwork, titles, func(library.Item) bool { return true })
		}
		return result
	})
}

func countLibrary(view *library.View) libraryCounts {
	counts := libraryCounts{movies: len(view.Movies()), shows: len(view.Shows())}
	for _, item := range view.Items() {
		switch item.Kind {
		case "photo":
			counts.photos++
		case "audio":
			counts.music++
		case "audiobook":
			counts.audiobooks++
		case "book":
			counts.books++
		}
	}
	return counts
}

func unwatchedShelf(cached *landing, watched func(library.Item) bool, kind string) []ShelfItem {
	if watched == nil {
		return nil
	}
	return recentShelf(cached.added[kind], cached.artwork, cached.titles, func(item library.Item) bool { return !watched(item) })
}

func homeShelves(cached *landing, watched func(library.Item) bool) []HomeShelf {
	shelves := []HomeShelf{
		{Key: "recent-movies", Title: "Recently added movies", Items: slices.Clone(cached.shelves["movie"])},
		{Key: "recent-shows", Title: "Recently added TV shows", Items: slices.Clone(cached.shelves["show"])},
		{Key: "unwatched-shows", Title: "Unwatched TV shows", Items: unwatchedShelf(cached, watched, "show")},
		{Key: "unwatched-movies", Title: "Unwatched movies", Items: unwatchedShelf(cached, watched, "movie")},
		{Key: "movie-genres", Title: "Movie genres", Genres: cached.genres},
		{Key: "recent-music", Title: "Recently added music", Items: slices.Clone(cached.shelves["audio"])},
		{Key: "recent-audiobooks", Title: "Recently added audiobooks", Items: slices.Clone(cached.shelves["audiobook"])},
		{Key: "recent-other", Title: "Recently added", Items: slices.Clone(cached.shelves["other"])},
	}
	visible := shelves[:0]
	for _, shelf := range shelves {
		if len(shelf.Items) > 0 || len(shelf.Genres) > 0 {
			visible = append(visible, shelf)
		}
	}
	for index := range visible {
		for card := range visible[index].Items {
			visible[index].Items[card].Priority = index == 0 && card < 2
		}
	}
	return visible
}
