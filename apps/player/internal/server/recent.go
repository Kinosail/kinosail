package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	homeview "github.com/MikeO7/kinosail/packages/home"
	"github.com/MikeO7/kinosail/packages/library"
)

func (source homeSource) Personal(request *http.Request, page catalog.Result, view *library.View) homeview.PersonalProjection[resumeItem] {
	shelves := page.ViewerShelves(source.progress.BrowseAccess(&source.lists.mu, &source.lists.values, request, viewerPolicy(currentViewer(request)).Allows))
	return homeview.PersonalProjection[resumeItem]{
		Continue:        shelves.Active,
		List:            shelves.Listed,
		Played:          shelves.Recent,
		Watched:         func(item library.Item) bool { return source.progress.Watched(request, item.ID) },
		CollectionCount: len(source.lists.collectionNamesFor(view)),
		PlaylistCount:   len(source.lists.PlaylistNames(request)),
	}
}

func (homeSource) HasArtwork(item library.Item) bool { return artworkPath(item) != "" }

func (source homeSource) Library(request *http.Request) *library.View {
	view, _ := visibleView(request, source.index)
	return view
}
