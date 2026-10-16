package viewing

import (
	"maps"
	"sort"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

type ProgressStorage struct {
	Mutex        *sync.RWMutex
	PersistMutex *sync.Mutex
	Values       *map[string]catalog.PlaybackState
	File         string
	Persist      func(string, any) error
	Database     *documentdb.Store
	Entries      bool
	ProfileID    string
	Owner        bool
}

const progressDocument = "progress.json"

func NewProgressStorage(storage catalog.ProgressStorage, database *documentdb.Store) ProgressStorage {
	return ProgressStorage{Mutex: storage.Mutex, PersistMutex: storage.PersistMutex, Values: storage.Values, File: storage.File, Persist: storage.Persist, Database: database, Entries: database != nil && storage.PersistEntry != nil}
}

func (storage ProgressStorage) entryWrites() bool {
	return storage.Entries && storage.Database != nil && storage.File != ""
}

func (storage ProgressStorage) Import(changes []ProgressChange, now time.Time) (int, int, error) {
	storage.PersistMutex.Lock()
	defer storage.PersistMutex.Unlock()
	storage.Mutex.Lock()
	defer storage.Mutex.Unlock()
	updates, applied, conflicts := progressUpdates(*storage.Values, storage.ProfileID, storage.Owner, changes, now)
	if applied > 0 && storage.File != "" {
		var err error
		if storage.entryWrites() {
			err = storage.Database.SaveEntries(progressDocument, entryValues(updates), nil)
		} else {
			err = storage.Persist(storage.File, mergedProgress(*storage.Values, updates))
		}
		if err != nil {
			return 0, conflicts, err
		}
	}
	publishProgress(storage.Values, updates)
	return applied, conflicts, nil
}

func MergeProgress(values map[string]catalog.PlaybackState, profileID string, owner bool, changes []ProgressChange, now time.Time) (map[string]catalog.PlaybackState, int, int) {
	updates, applied, conflicts := progressUpdates(values, profileID, owner, changes, now)
	return mergedProgress(values, updates), applied, conflicts
}

func progressUpdates(values map[string]catalog.PlaybackState, profileID string, owner bool, changes []ProgressChange, now time.Time) (map[string]catalog.PlaybackState, int, int) {
	updates, applied, conflicts := make(map[string]catalog.PlaybackState), 0, 0
	for _, change := range changes {
		key := profileID + ":" + change.TargetID
		current, found := updates[key]
		if !found {
			current, found = values[key]
		}
		if !found && owner {
			current = values[change.TargetID]
		}
		if current == change.Expected {
			state := change.State
			if state.Updated.IsZero() {
				state.Updated = now.UTC()
			}
			state.Dismissed, state.Session, state.Revision = false, "", 0
			updates[key], applied = state, applied+1
		} else {
			conflicts++
		}
	}
	return updates, applied, conflicts
}

func mergedProgress(values, updates map[string]catalog.PlaybackState) map[string]catalog.PlaybackState {
	result := maps.Clone(values)
	if result == nil {
		result = make(map[string]catalog.PlaybackState, len(updates))
	}
	maps.Copy(result, updates)
	return result
}

func publishProgress(values *map[string]catalog.PlaybackState, updates map[string]catalog.PlaybackState) {
	if *values == nil {
		*values = make(map[string]catalog.PlaybackState, len(updates))
	}
	maps.Copy(*values, updates)
}

func entryValues(updates map[string]catalog.PlaybackState) map[string]any {
	values := make(map[string]any, len(updates))
	for key, state := range updates {
		values[key] = state
	}
	return values
}

func MergeLists(values map[string]bool, playlists map[string]map[string]bool, order map[string][]string, smart map[string]catalog.PlaylistRule, viewer string, changes []ListChange) (map[string]bool, map[string]map[string]bool, map[string][]string, int) {
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
