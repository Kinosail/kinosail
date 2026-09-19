package catalog

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// CreateCollection persists one named Collection. The caller must hold its storage lock.
func (storage ListStorage) CreateCollection(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if !ValidListName(name) {
		return errors.New("collection name must contain 1 to 64 characters")
	}
	state := storage.State()
	key := collectionStateKey(name)
	if state.Playlists[key] == nil {
		state.Playlists[key] = make(map[string]bool)
	}
	delete(state.Playlists[key], "!collection")
	return storage.Commit(ctx, state, PlaylistsDocument)
}

// SetCollection changes one Collection membership. The caller must hold its storage lock.
func (storage ListStorage) SetCollection(ctx context.Context, name string, item library.Item, included bool) error {
	state := storage.State()
	collection := state.Playlists[collectionStateKey(name)]
	if collection == nil && item.Collection == name {
		collection = make(map[string]bool)
		state.Playlists[collectionStateKey(name)] = collection
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
	return storage.Commit(ctx, state, PlaylistsDocument)
}

// DeleteCollection hides one Collection and persists all visible-item exclusions. The caller must hold its storage lock.
func (storage ListStorage) DeleteCollection(ctx context.Context, name string, items []library.Item) error {
	state := storage.State()
	key := collectionStateKey(name)
	collection := state.Playlists[key]
	if collection == nil {
		collection = make(map[string]bool)
		state.Playlists[key] = collection
	}
	for _, item := range items {
		delete(collection, item.ID)
		collection["!"+item.ID] = item.Collection == name
	}
	collection["!collection"] = true
	return storage.Commit(ctx, state, PlaylistsDocument)
}

func collectionStateKey(name string) string { return "collection:" + name }
