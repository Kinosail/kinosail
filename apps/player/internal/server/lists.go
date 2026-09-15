package server

import (
	"context"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
	"github.com/MikeO7/kinosail/packages/library"
)

type listStore struct {
	mu                sync.RWMutex
	file              string
	values            map[string]bool
	playlistsFile     string
	playlists         map[string]map[string]bool
	playlistOrderFile string
	playlistOrder     map[string][]string
	smartFile         string
	smart             map[string]playlistRule
	persist           func(string, any) error
	database          *database.Store
	err               error
}

func (store *listStore) ExportPlaylistDocument(viewer, name string) (catalogapi.PlaylistDocument, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	key := viewer + ":" + name
	return catalogapi.ExportPlaylistDocument(name, store.playlists[key], store.playlistOrder[key], store.smart[key].Sort != "")
}

type playlistRule = catalog.PlaylistRule

func newListStore(dataDir string, databases ...*database.Store) *listStore {
	stateDB := configuredDatabase(databases)
	store := &listStore{values: make(map[string]bool), playlists: make(map[string]map[string]bool), playlistOrder: make(map[string][]string), smart: make(map[string]playlistRule), persist: statePersistence(stateDB), database: stateDB}
	if dataDir != "" {
		store.file = filepath.Join(dataDir, "lists.json")
		store.playlistsFile = filepath.Join(dataDir, "playlists.json")
		store.playlistOrderFile = filepath.Join(dataDir, "playlist_order.json")
		store.smartFile = filepath.Join(dataDir, "smart_playlists.json")
		store.err = store.storage().Restore()
	}
	return store
}

func (store *listStore) storage() catalog.ListStorage {
	return catalog.ListStorage{Values: &store.values, Playlists: &store.playlists, PlaylistOrder: &store.playlistOrder, Smart: &store.smart, Paths: catalog.ListPaths{Values: store.file, Playlists: store.playlistsFile, PlaylistOrder: store.playlistOrderFile, Smart: store.smartFile}, Database: store.database, Persist: store.persist, Load: func(file string, target any) (bool, error) { return loadState(store.database, file, target) }, LoadError: &store.err}
}

func (store *listStore) PlaylistNames(request *http.Request) []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().PlaylistNames(currentViewer(request).ID)
}

func (store *listStore) editablePlaylistNames(request *http.Request) []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().EditablePlaylistNames(currentViewer(request).ID)
}

func (store *listStore) Playlist(request *http.Request, name string, items []library.Item) []library.Item {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.storage().Playlist(currentViewer(request).ID, name, items)
}

func (store *listStore) CreateSmart(ctx context.Context, viewer, name string, rule playlistRule) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().CreateSmart(ctx, viewer, name, rule)
}

func (store *listStore) Has(request *http.Request, id string) bool {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.values[currentViewer(request).ID+":"+id]
}

func (store *listStore) Create(ctx context.Context, viewer, name string, ids ...string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().Create(ctx, viewer, name, ids...)
}

func (store *listStore) SetPlaylist(ctx context.Context, viewer, name, id string, included bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().SetPlaylist(ctx, viewer, name, id, included)
}

func (store *listStore) Order(ctx context.Context, viewer, name string, ids []string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().Order(ctx, viewer, name, ids)
}

func (store *listStore) DeletePlaylist(ctx context.Context, viewer, name string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().DeletePlaylist(ctx, viewer, name)
}

func (store *listStore) SetListed(ctx context.Context, viewer, id string, listed bool) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.storage().SetListed(ctx, viewer, id, listed)
}
