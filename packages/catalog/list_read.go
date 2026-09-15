package catalog

import (
	"slices"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// PlaylistNames returns the viewer's manual and smart playlists in name order.
// The caller must hold its state read lock for all ListStorage read operations.
func (storage ListStorage) PlaylistNames(viewer string) []string {
	prefix := viewer + ":"
	names := storage.EditablePlaylistNames(viewer)
	for key := range *storage.Smart {
		if strings.HasPrefix(key, prefix) {
			names = append(names, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(names)
	return names
}

// EditablePlaylistNames excludes collections and smart playlists.
func (storage ListStorage) EditablePlaylistNames(viewer string) []string {
	prefix := viewer + ":"
	names := make([]string, 0)
	for key := range *storage.Playlists {
		if strings.HasPrefix(key, prefix) && !strings.HasPrefix(strings.TrimPrefix(key, prefix), "collection:") {
			names = append(names, strings.TrimPrefix(key, prefix))
		}
	}
	sort.Strings(names)
	return names
}

// Playlist selects visible members using Player's manual ordering or smart rule.
func (storage ListStorage) Playlist(viewer, name string, items []library.Item) []library.Item { //nolint:cyclop // Ordered and smart playlists share one small read path.
	key := viewer + ":" + name
	if rule, ok := (*storage.Smart)[key]; ok {
		selected := Filter(items, rule.Query)
		if rule.Kind == "audio" {
			selected = append(Media(selected, "audio"), Media(selected, "audiobook")...)
		} else if rule.Kind != "" {
			selected = Media(selected, rule.Kind)
		}
		selected = Sort(slices.Clone(selected), rule.Sort)
		return selected
	}
	selected := (*storage.Playlists)[key]
	byID := make(map[string]library.Item, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	result := make([]library.Item, 0)
	for _, id := range (*storage.PlaylistOrder)[key] {
		if item, ok := byID[id]; ok && selected[id] {
			result = append(result, item)
			delete(byID, id)
		}
	}
	for _, item := range items {
		if _, ok := byID[item.ID]; ok && selected[item.ID] {
			result = append(result, item)
		}
	}
	return result
}
