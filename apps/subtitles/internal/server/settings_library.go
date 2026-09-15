package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

type libraryIndex struct {
	*catalog.Index
}
type libraryRoot = catalog.ScanRoot

func (index *libraryIndex) VisibleLibrary(request *http.Request) []library.Item {
	items, _ := visibleLibrary(request, index)
	return items
}

func (index *libraryIndex) VisibleItem(request *http.Request, id string) (library.Item, bool) {
	return visibleItem(request, index, id)
}

type libraryFolderState struct{ store *settingsStore }
