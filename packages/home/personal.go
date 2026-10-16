package home

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/library"
)

func Personal[Resume any](
	request *http.Request,
	items []library.Item,
	active func(*http.Request, []library.Item) []Resume,
	listed func(*http.Request, []library.Item) []library.Item,
	played func(*http.Request, []library.Item) []library.Item,
	collections func([]library.Item) []string,
	playlists func(*http.Request) []string,
) PersonalProjection[Resume] {
	list := listed(request, items)
	return PersonalProjection[Resume]{
		Continue:        active(request, items),
		List:            list,
		Played:          played(request, items),
		CollectionCount: len(collections(items)),
		PlaylistCount:   len(playlists(request)),
	}
}
