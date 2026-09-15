package home

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

// MediaPresentation selects visible artwork and Show titles for home shelves.
func MediaPresentation(items []library.Item, hasArtwork func(library.Item) bool) (map[string]string, map[string]string) {
	artwork, titles := make(map[string]string), make(map[string]string)
	for _, item := range items {
		if hasArtwork(item) {
			artwork[item.ID] = item.ID
		}
	}
	_, shows := library.Organize(items)
	for _, show := range shows {
		for _, episode := range show.Episodes {
			titles[episode.ID] = show.Title
			if artwork[episode.ID] == "" {
				artwork[episode.ID] = show.ArtworkID
			}
		}
	}
	return artwork, titles
}

// ClearSearchURL removes search-only query state from a Library URL.
func ClearSearchURL(request *http.Request) string {
	query := request.URL.Query()
	query.Del("q")
	query.Del("offset")
	query.Del("letter")
	if len(query) == 0 {
		return "/"
	}
	return "/?" + query.Encode()
}

// FragmentRequest validates the bounded Library fragment header.
func FragmentRequest(request *http.Request) (bool, error) {
	values := request.Header.Values(InfiniteLibraryHeader)
	if len(values) > 1 || len(values) == 1 && values[0] != "1" {
		return false, catalog.ErrInvalidBrowse
	}
	return len(values) == 1, nil
}

// Media returns items with one exact canonical kind.
func Media(items []library.Item, kind string) []library.Item {
	return catalog.Media(items, kind)
}
