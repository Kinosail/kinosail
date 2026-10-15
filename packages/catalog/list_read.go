package catalog

import (
	"slices"
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

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

func (storage ListStorage) Playlist(viewer, name string, items []library.Item) []library.Item {
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
