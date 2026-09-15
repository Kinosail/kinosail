package catalog

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
)

// CreateSmart validates and persists a Player smart playlist. The caller must hold its storage lock.
func (storage ListStorage) CreateSmart(ctx context.Context, viewer, name string, rule PlaylistRule) error {
	name, rule.Query = strings.TrimSpace(name), strings.TrimSpace(rule.Query)
	if !ValidListName(name) || len(rule.Query) > 200 || !oneOf(rule.Kind, "", "video", "audio", "book", "photo") || !oneOf(rule.Sort, "title", "added", "year") {
		return errors.New("invalid smart playlist rule")
	}
	state := storage.State()
	key := viewer + ":" + name
	if state.Playlists[key] != nil || state.Smart[key].Sort != "" {
		return errors.New("playlist name already exists")
	}
	state.Smart[key] = rule
	return storage.Commit(ctx, state, SmartListDocument)
}

// Create persists one validated playlist and its order. The caller must hold its storage lock.
func (storage ListStorage) Create(ctx context.Context, viewer, name string, ids ...string) error {
	name, err := ValidatePlaylist(name, ids)
	if err != nil {
		return err
	}
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		selected[id] = true
	}
	state := storage.State()
	key := viewer + ":" + name
	if state.Smart[key].Sort != "" {
		return errors.New("playlist name already exists")
	}
	if state.Playlists[key] == nil {
		state.Playlists[key] = selected
		state.PlaylistOrder[key] = append([]string(nil), ids...)
	} else if len(ids) != 0 {
		return errors.New("playlist name already exists")
	}
	return storage.Commit(ctx, state, PlaylistsDocument|PlaylistOrderDocument)
}

// SetPlaylist changes membership without duplicate order entries. The caller must hold its storage lock.
func (storage ListStorage) SetPlaylist(ctx context.Context, viewer, name, id string, included bool) error {
	state := storage.State()
	key := viewer + ":" + name
	playlist := state.Playlists[key]
	if playlist == nil {
		return os.ErrNotExist
	}
	playlist[id] = included
	if !included {
		delete(playlist, id)
		state.PlaylistOrder[key] = slices.DeleteFunc(state.PlaylistOrder[key], func(value string) bool { return value == id })
	} else if !slices.Contains(state.PlaylistOrder[key], id) {
		state.PlaylistOrder[key] = append(state.PlaylistOrder[key], id)
	}
	return storage.Commit(ctx, state, PlaylistsDocument|PlaylistOrderDocument)
}

// Order requires every selected item exactly once. The caller must hold its storage lock.
func (storage ListStorage) Order(ctx context.Context, viewer, name string, ids []string) error {
	state := storage.State()
	key, selected := viewer+":"+name, state.Playlists[viewer+":"+name]
	if selected == nil || len(ids) != len(selected) {
		return errors.New("order must contain every playlist item")
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !selected[id] || seen[id] {
			return errors.New("order must contain every playlist item")
		}
		seen[id] = true
	}
	state.PlaylistOrder[key] = append([]string(nil), ids...)
	return storage.Commit(ctx, state, PlaylistOrderDocument)
}

// DeletePlaylist removes only the viewer's named playlist. The caller must hold its storage lock.
func (storage ListStorage) DeletePlaylist(ctx context.Context, viewer, name string) error {
	state := storage.State()
	delete(state.Playlists, viewer+":"+name)
	delete(state.PlaylistOrder, viewer+":"+name)
	delete(state.Smart, viewer+":"+name)
	return storage.Commit(ctx, state, PlaylistsDocument|PlaylistOrderDocument|SmartListDocument)
}

// SetListed persists the viewer's My List choice. The caller must hold its storage lock.
func (storage ListStorage) SetListed(ctx context.Context, viewer, id string, listed bool) error {
	state := storage.State()
	state.Values[viewer+":"+id] = listed
	return storage.Commit(ctx, state, ListedDocument)
}

// ValidatePlaylist validates one named playlist and its stable item order.
func ValidatePlaylist(name string, ids []string) (string, error) {
	name = strings.TrimSpace(name)
	if !ValidListName(name) {
		return "", errors.New("name must contain 1 to 64 characters")
	}
	if len(ids) > 200 {
		return "", errors.New("playlist may contain at most 200 items")
	}
	selected := make(map[string]bool, len(ids))
	for _, id := range ids {
		if id == "" || len(id) > 256 {
			return "", errors.New("playlist item IDs must contain 1 to 256 characters")
		}
		if selected[id] {
			return "", errors.New("playlist items must be unique")
		}
		selected[id] = true
	}
	return name, nil
}
