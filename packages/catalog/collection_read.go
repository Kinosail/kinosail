package catalog

import (
	"sort"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// CollectionNames returns built-in and custom collection names for visible items.
func (storage ListStorage) CollectionNames(items []library.Item) []string {
	values := make([]string, 0)
	for _, item := range items {
		if item.Collection != "" {
			values = append(values, item.Collection)
		}
	}
	return storage.collectionNamesWith(values)
}

// collectionNamesWith merges built-in names with stored collection visibility.
func (storage ListStorage) collectionNamesWith(values []string) []string {
	unique := make(map[string]bool)
	for key := range *storage.Playlists {
		if strings.HasPrefix(key, "collection:") && !(*storage.Playlists)[key]["!collection"] {
			unique[strings.TrimPrefix(key, "collection:")] = true
		}
	}
	for _, name := range values {
		if !(*storage.Playlists)["collection:"+name]["!collection"] {
			unique[name] = true
		}
	}
	names := make([]string, 0, len(unique))
	for name := range unique {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Collection selects built-in and explicitly included members, honoring exclusions.
func (storage ListStorage) Collection(name string, items []library.Item) []library.Item {
	selected, result := (*storage.Playlists)["collection:"+name], make([]library.Item, 0)
	for _, item := range items {
		if !selected["!"+item.ID] && (selected[item.ID] || item.Collection == name) {
			result = append(result, item)
		}
	}
	return result
}
