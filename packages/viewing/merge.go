package viewing

import (
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

// ProgressStorage connects viewing-import reconciliation to an app-owned state map.
type ProgressStorage struct {
	Mutex, PersistMutex *sync.Mutex
	Values              *map[string]catalog.PlaybackState
	File                string
	Persist             func(string, any) error
	Database            *documentdb.Store
	ProfileID           string
	Owner               bool
}

// NewProgressStorage adapts the shared progress transaction to viewing-import reconciliation.
func NewProgressStorage(storage catalog.ProgressStorage, database *documentdb.Store) ProgressStorage {
	return ProgressStorage{Mutex: storage.Mutex, PersistMutex: storage.PersistMutex, Values: storage.Values, File: storage.File, Persist: storage.Persist, Database: database}
}

// Import atomically persists and publishes a batch of optimistic viewing changes.
func (storage ProgressStorage) Import(changes []ProgressChange, now time.Time) (int, int, error) {
	storage.PersistMutex.Lock()
	defer storage.PersistMutex.Unlock()
	storage.Mutex.Lock()
	defer storage.Mutex.Unlock()
	values, applied, conflicts := MergeProgress(*storage.Values, storage.ProfileID, storage.Owner, changes, now)
	if applied > 0 && storage.File != "" {
		if err := storage.Persist(storage.File, values); err != nil {
			return 0, conflicts, err
		}
	}
	*storage.Values = values
	return applied, conflicts, nil
}

// MergeProgress applies optimistic viewing changes to a detached state map.
func MergeProgress(values map[string]catalog.PlaybackState, profileID string, owner bool, changes []ProgressChange, now time.Time) (map[string]catalog.PlaybackState, int, int) {
	result, applied, conflicts := maps.Clone(values), 0, 0
	for _, change := range changes {
		key := profileID + ":" + change.TargetID
		current, found := result[key]
		if !found && owner {
			current = result[change.TargetID]
		}
		if current == change.Expected {
			state := change.State
			if state.Updated.IsZero() {
				state.Updated = now.UTC()
			}
			state.Dismissed, state.Session, state.Revision = false, "", 0
			result[key], applied = state, applied+1
		} else {
			conflicts++
		}
	}
	return result, applied, conflicts
}

// MergeLists adds favorites and ordered playlist entries to detached maps.
func MergeLists(values map[string]bool, playlists map[string]map[string]bool, order map[string][]string, smart map[string]catalog.PlaylistRule, viewer string, changes []ListChange) (map[string]bool, map[string]map[string]bool, map[string][]string, int) { //nolint:cyclop,gocognit // Favorite and ordered playlist merges form one optimistic transaction.
	type positionedItem struct {
		id       string
		position int
	}
	pending, applied := make(map[string][]positionedItem), 0
	for _, change := range changes {
		if key := viewer + ":" + change.TargetID; change.Favorite && !values[key] {
			values[key], applied = true, applied+1
		}
		for name, position := range change.Playlists {
			key := viewer + ":" + name
			if catalog.ValidListName(name) && smart[key].Sort == "" && !playlists[key][change.TargetID] {
				if playlists[key] == nil {
					playlists[key] = make(map[string]bool)
				}
				playlists[key][change.TargetID], applied = true, applied+1
				pending[key] = append(pending[key], positionedItem{change.TargetID, position})
			}
		}
	}
	for key, items := range pending {
		sort.SliceStable(items, func(left, right int) bool { return items[left].position < items[right].position })
		for _, item := range items {
			order[key] = append(order[key], item.id)
		}
	}
	return values, playlists, order, applied
}
