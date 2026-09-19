package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playerweb"
)

var curationTemplate = strings.NewReplacer("Kinosail Player", "Kinosail Subtitles", "theme.js?v=6", "theme.js?v=4", "app.css?v=81", "app.css?v=81")

var collectionHTML = curationTemplate.Replace(playerweb.CollectionHTML)

var collectionView = newLocalizedTemplate("collection", collectionHTML)

func (store *listStore) collectionNames(items []library.Item) []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().CollectionNames(items)
}

func (store *listStore) collection(name string, items []library.Item) []library.Item {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().Collection(name, items)
}

func (store *listStore) CollectionNames(items []library.Item) []string {
	return store.collectionNames(items)
}

func (store *listStore) Collection(name string, items []library.Item) []library.Item {
	return store.collection(name, items)
}

func (store *listStore) CreateCollection(ctx context.Context, name string) error {
	return store.createCollection(ctx, name)
}

func (store *listStore) SetCollection(ctx context.Context, name string, item library.Item, included bool) error {
	return store.setCollection(ctx, name, item, included)
}

func (store *listStore) DeleteCollection(ctx context.Context, name string, items []library.Item) error {
	return store.deleteCollection(ctx, name, items)
}

func (store *listStore) createCollection(ctx context.Context, name string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().CreateCollection(ctx, name)
}

func (store *listStore) setCollection(ctx context.Context, name string, item library.Item, included bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().SetCollection(ctx, name, item, included)
}

func (store *listStore) deleteCollection(ctx context.Context, name string, items []library.Item) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().DeleteCollection(ctx, name, items)
}

func collectionHandlers(index *libraryIndex, store *listStore) catalog.CollectionWebHandlers {
	return catalog.NewCollectionWebHandlersForStore(store, catalog.CollectionWebApp{
		Index: index,
		Owner: func(request *http.Request) bool {
			return currentViewer(request).Owner
		},
		Render: func(writer http.ResponseWriter, request *http.Request, page catalog.CollectionWebPage) error {
			return collectionView.Execute(writer, request, page)
		},
		Failure:  localizedError,
		NotFound: localizedNotFound,
	})
}

func registerCollections(mux *http.ServeMux, index *libraryIndex, store *listStore, auth *authentication) {
	mux.Handle("POST /collections", auth.owner(createCollection(index, store)))
	mux.Handle("POST /collection/{name}/{id}", auth.owner(saveCollection(index, store)))
	mux.Handle("POST /collection/{name}/items/{id}", auth.owner(manageCollectionItem(index, store)))
	mux.Handle("POST /collection/{name}/delete", auth.owner(deleteCollection(index, store)))
	mux.HandleFunc("GET /collection/{name}", browseCollection(index, store))
}

func manageCollectionItem(index *libraryIndex, store *listStore) http.HandlerFunc {
	return collectionHandlers(index, store).ManageItem
}

func createCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return collectionHandlers(index, store).Create
}

func saveCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return collectionHandlers(index, store).Save
}

func deleteCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return collectionHandlers(index, store).Delete
}

func browseCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return collectionHandlers(index, store).Browse
}
