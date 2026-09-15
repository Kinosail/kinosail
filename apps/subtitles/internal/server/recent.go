package server

import (
	"net/http"

	homeview "github.com/MikeO7/kinosail/packages/home"
	"github.com/MikeO7/kinosail/packages/library"
)

func (source homeSource) Personal(request *http.Request, items []library.Item) homeview.PersonalProjection[resumeItem] {
	return homeview.Personal(request, items, source.progress.Active, source.lists.items, source.progress.Recent, source.lists.collectionNames, source.lists.PlaylistNames)
}

func (homeSource) HasArtwork(item library.Item) bool { return artworkPath(item) != "" }
