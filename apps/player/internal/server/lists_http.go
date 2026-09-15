package server

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
)

func (store *listStore) items(request *http.Request, items []library.Item) []library.Item {
	return catalog.ListedItems(items, func(id string) bool { return store.Has(request, id) })
}
