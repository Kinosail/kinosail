package server

import "github.com/MikeO7/kinosail/packages/catalog"

func (store *listStore) viewingImportable(viewer, id string, favorite bool, playlists map[string]int) int {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return catalog.ListAdditions(catalog.ListState{Values: store.values, Playlists: store.playlists, Smart: store.smart}, viewer, id, favorite, playlists)
}
