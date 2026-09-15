package server

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playerweb"
)

const collectionHTML = playerweb.CollectionHTML

var collectionView = newLocalizedTemplate("collection", collectionHTML)

func collectionKey(name string) string { return "collection:" + name }

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
	name = strings.TrimSpace(name)
	if !validListName(name) {
		return errors.New("collection name must contain 1 to 64 characters")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	state := store.storage().State()
	if state.Playlists[collectionKey(name)] == nil {
		state.Playlists[collectionKey(name)] = make(map[string]bool)
	}
	delete(state.Playlists[collectionKey(name)], "!collection")
	return store.storage().Commit(ctx, state, catalog.PlaylistsDocument)
}

func (store *listStore) setCollection(ctx context.Context, name string, item library.Item, included bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	state := store.storage().State()
	collection := state.Playlists[collectionKey(name)]
	if collection == nil && item.Collection == name {
		collection = make(map[string]bool)
		state.Playlists[collectionKey(name)] = collection
	}
	if collection == nil || collection["!collection"] {
		return os.ErrNotExist
	}
	collection[item.ID] = included
	delete(collection, "!"+item.ID)
	if !included {
		delete(collection, item.ID)
		collection["!"+item.ID] = item.Collection == name
	}
	return store.storage().Commit(ctx, state, catalog.PlaylistsDocument)
}

func (store *listStore) deleteCollection(ctx context.Context, name string, items []library.Item) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	state := store.storage().State()
	collection := state.Playlists[collectionKey(name)]
	if collection == nil {
		collection = make(map[string]bool)
		state.Playlists[collectionKey(name)] = collection
	}
	for _, item := range items {
		delete(collection, item.ID)
		collection["!"+item.ID] = item.Collection == name
	}
	collection["!collection"] = true
	return store.storage().Commit(ctx, state, catalog.PlaylistsDocument)
}

func registerCollections(mux *http.ServeMux, index *libraryIndex, store *listStore, auth *authentication) {
	mux.Handle("POST /collections", auth.owner(createCollection(store)))
	mux.Handle("POST /collection/{name}/{id}", auth.owner(saveCollection(index, store)))
	mux.Handle("POST /collection/{name}/items/{id}", auth.owner(manageCollectionItem(index, store)))
	mux.Handle("POST /collection/{name}/delete", auth.owner(deleteCollection(index, store)))
	mux.HandleFunc("GET /collection/{name}", browseCollection(index, store))
}

func manageCollectionItem(index *libraryIndex, store *listStore) http.HandlerFunc { //nolint:gocognit,cyclop // Strict validation and no-side-effect failures stay in one adapter.
	return func(writer http.ResponseWriter, request *http.Request) {
		if len(request.URL.Query()) != 0 {
			localizedError(writer, request, "invalid Collection state", http.StatusBadRequest)
			return
		}
		if err := request.ParseForm(); err != nil || !validCurationItemForm(request.PostForm) {
			localizedError(writer, request, "invalid Collection state", http.StatusBadRequest)
			return
		}
		name, id := request.PathValue("name"), request.PathValue("id")
		if !validCollectionPathName(name) || !validCurationItemID(id) {
			localizedError(writer, request, "invalid Collection state", http.StatusBadRequest)
			return
		}
		included, _ := strconv.ParseBool(request.PostForm.Get("included"))
		item, found := visibleItem(request, index, id)
		if !found {
			localizedNotFound(writer, request)
			return
		}
		if err := store.setCollection(request.Context(), name, item, included); errors.Is(err, os.ErrNotExist) {
			localizedNotFound(writer, request)
			return
		} else if err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		target := "/collection/" + url.PathEscape(name)
		if query := strings.TrimSpace(request.PostForm.Get("q")); included && query != "" {
			target += "?q=" + url.QueryEscape(query) + "#collection-search"
		}
		http.Redirect(writer, request, target, http.StatusSeeOther)
	}
}

func createCollection(store *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		name := strings.TrimSpace(request.FormValue("name"))
		if err := store.createCollection(request.Context(), name); err != nil {
			status := http.StatusInternalServerError
			if !validListName(name) {
				status = http.StatusBadRequest
			}
			localizedError(writer, request, err.Error(), status)
			return
		}
		http.Redirect(writer, request, "/collection/"+url.PathEscape(name), http.StatusSeeOther)
	}
}

func saveCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		item, found := visibleItem(request, index, request.PathValue("id"))
		included, err := strconv.ParseBool(request.FormValue("included"))
		if !found || err != nil {
			localizedError(writer, request, "invalid Collection state", http.StatusBadRequest)
			return
		}
		err = store.setCollection(request.Context(), request.PathValue("name"), item, included)
		if errors.Is(err, os.ErrNotExist) {
			localizedNotFound(writer, request)
			return
		}
		if err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		target := "/watch/"
		if item.Kind == "book" {
			target = "/book/"
		}
		http.Redirect(writer, request, target+item.ID, http.StatusSeeOther)
	}
}

func deleteCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		items, _ := index.Snapshot()
		if err := store.deleteCollection(request.Context(), request.PathValue("name"), items); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(writer, request, "/", http.StatusSeeOther)
	}
}

func browseCollection(index *libraryIndex, store *listStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		query, err := curationQuery(request)
		if err != nil {
			localizedError(writer, request, "invalid Collection state", http.StatusBadRequest)
			return
		}
		name := request.PathValue("name")
		items, _ := visibleLibrary(request, index)
		if !contains(store.collectionNames(items), name) {
			localizedNotFound(writer, request)
			return
		}
		writer.Header().Set("Content-Type", "text/html; charset=utf-8")
		members := store.collection(name, items)
		candidates := collectionCandidates(items, members, query)
		if err := collectionView.Execute(writer, request, struct {
			Name       string
			Query      string
			Items      []library.Item
			Candidates []library.Item
			ItemCount  int
			Owner      bool
		}{name, query, members, candidates, len(members), currentViewer(request).Owner}); err != nil {
			localizedError(writer, request, err.Error(), http.StatusInternalServerError)
		}
	}
}

func curationQuery(request *http.Request) (string, error) {
	for key := range request.URL.Query() {
		if key != "q" && key != "lang" {
			return "", catalog.ErrInvalidBrowse
		}
	}
	return catalog.SingleValue(request.URL.Query(), "q", 200)
}

func collectionCandidates(items, members []library.Item, query string) []library.Item {
	if query == "" {
		return nil
	}
	included := make(map[string]bool, len(members))
	for _, item := range members {
		included[item.ID] = true
	}
	candidates := make([]library.Item, 0)
	for _, item := range filter(items, query) {
		if !included[item.ID] {
			candidates = append(candidates, item)
		}
	}
	candidates = sortLibrary(candidates, "title")
	return candidates[:min(48, len(candidates))]
}
