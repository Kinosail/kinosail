package server

import (
	"net/http"

	homeview "github.com/MikeO7/kinosail/packages/home"
	"github.com/MikeO7/kinosail/packages/library"
)

func (source homeSource) Personal(request *http.Request, items []library.Item) homeview.PersonalProjection[resumeItem] {
	personal := homeview.Personal(request, items, source.progress.Active, source.lists.items, source.progress.Recent, source.lists.collectionNames, source.lists.PlaylistNames)
	for _, item := range items {
		if item.Kind == "video" && !source.progress.Watched(request, item.ID) {
			personal.Unwatched = append(personal.Unwatched, item)
		}
	}
	return personal
}

func (homeSource) HasArtwork(item library.Item) bool { return artworkPath(item) != "" }
