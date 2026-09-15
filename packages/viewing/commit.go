package viewing

import (
	"errors"
	"maps"
	"path/filepath"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

// CommitStore adapts app-private progress and list state to one atomic import.
type CommitStore struct {
	Progress  ProgressStorage
	Lists     catalog.ListStorage
	ListMutex *sync.RWMutex
}

type viewingWrite struct {
	file     string
	old, new any
	persist  func(string, any) error
}

// Commit validates optimistic changes and publishes every category together.
func (store CommitStore) Commit(profile Profile, progressChanges []ProgressChange, listChanges []ListChange) (int, int, int, error) { //nolint:cyclop,funlen,gocognit // One lock and rollback seam prevents partial category commits.
	store.Progress.PersistMutex.Lock()
	defer store.Progress.PersistMutex.Unlock()
	store.Progress.Mutex.Lock()
	defer store.Progress.Mutex.Unlock()
	store.ListMutex.Lock()
	defer store.ListMutex.Unlock()

	oldProgress := maps.Clone(*store.Progress.Values)
	newProgress, applied, conflicts := MergeProgress(oldProgress, profile.ID, profile.Owner, progressChanges, time.Now())
	oldLists := catalog.CloneListState(*store.Lists.Values, *store.Lists.Playlists, *store.Lists.PlaylistOrder, nil)
	newLists := catalog.CloneListState(oldLists.Values, oldLists.Playlists, oldLists.PlaylistOrder, nil)
	newValues, newPlaylists, newOrder, listsApplied := MergeLists(newLists.Values, newLists.Playlists, newLists.PlaylistOrder, *store.Lists.Smart, profile.ID, listChanges)
	writes := make([]viewingWrite, 0, 4)
	if applied > 0 {
		writes = append(writes, viewingWrite{store.Progress.File, oldProgress, newProgress, store.Progress.Persist})
	}
	if listsApplied > 0 {
		writes = append(writes,
			viewingWrite{store.Lists.Paths.Values, oldLists.Values, newValues, store.Lists.Persist},
			viewingWrite{store.Lists.Paths.Playlists, oldLists.Playlists, newPlaylists, store.Lists.Persist},
			viewingWrite{store.Lists.Paths.PlaylistOrder, oldLists.PlaylistOrder, newOrder, store.Lists.Persist},
		)
	}
	if store.Progress.Database != nil && store.Progress.Database == store.Lists.Database {
		documents := make(map[string]any, len(writes))
		for _, write := range writes {
			if write.file != "" {
				documents[filepath.Base(write.file)] = write.new
			}
		}
		if err := store.Progress.Database.SaveJSONBatch(documents); err != nil {
			return 0, conflicts, 0, err
		}
		writes = nil
	}
	done := make([]viewingWrite, 0, len(writes))
	for _, write := range writes {
		if write.file == "" || write.persist == nil {
			continue
		}
		if err := write.persist(write.file, write.new); err != nil {
			return 0, conflicts, 0, rollbackViewingWrites(done, err)
		}
		done = append(done, write)
	}
	*store.Progress.Values = newProgress
	*store.Lists.Values, *store.Lists.Playlists, *store.Lists.PlaylistOrder = newValues, newPlaylists, newOrder
	return applied, conflicts, listsApplied, nil
}

func rollbackViewingWrites(writes []viewingWrite, cause error) error {
	var rollbackErr error
	for index := len(writes) - 1; index >= 0; index-- {
		if err := writes[index].persist(writes[index].file, writes[index].old); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	if rollbackErr != nil {
		return errors.Join(cause, rollbackErr)
	}
	return cause
}
