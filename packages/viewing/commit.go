package viewing

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

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

func (store CommitStore) Commit(profile Profile, progressChanges []ProgressChange, listChanges []ListChange) (int, int, int, error) {
	store.Progress.PersistMutex.Lock()
	defer store.Progress.PersistMutex.Unlock()
	store.Progress.Mutex.Lock()
	defer store.Progress.Mutex.Unlock()
	store.ListMutex.Lock()
	defer store.ListMutex.Unlock()

	updates, applied, conflicts := progressUpdates(*store.Progress.Values, profile.ID, profile.Owner, progressChanges, time.Now())
	oldLists := catalog.CloneListState(*store.Lists.Values, *store.Lists.Playlists, *store.Lists.PlaylistOrder, nil)
	newLists := catalog.CloneListState(oldLists.Values, oldLists.Playlists, oldLists.PlaylistOrder, nil)
	newValues, newPlaylists, newOrder, listsApplied := MergeLists(newLists.Values, newLists.Playlists, newLists.PlaylistOrder, *store.Lists.Smart, profile.ID, listChanges)
	writes := make([]viewingWrite, 0, 3)
	if listsApplied > 0 {
		writes = append(writes,
			viewingWrite{store.Lists.Paths.Values, oldLists.Values, newValues, store.Lists.Persist},
			viewingWrite{store.Lists.Paths.Playlists, oldLists.Playlists, newPlaylists, store.Lists.Persist},
			viewingWrite{store.Lists.Paths.PlaylistOrder, oldLists.PlaylistOrder, newOrder, store.Lists.Persist},
		)
	}
	var err error
	if store.Progress.Database != nil && store.Progress.Database == store.Lists.Database {
		err = store.commitDatabase(updates, applied, writes)
	} else {
		err = store.commitFiles(updates, applied, writes)
	}
	if err != nil {
		return 0, conflicts, 0, err
	}
	publishProgress(store.Progress.Values, updates)
	*store.Lists.Values, *store.Lists.Playlists, *store.Lists.PlaylistOrder = newValues, newPlaylists, newOrder
	return applied, conflicts, listsApplied, nil
}

func (store CommitStore) commitDatabase(updates map[string]catalog.PlaybackState, applied int, writes []viewingWrite) error {
	batch := documentdb.Batch{Documents: make(map[string]any, len(writes)+1)}
	for _, write := range writes {
		if write.file != "" {
			batch.Documents[filepath.Base(write.file)] = write.new
		}
	}
	if applied > 0 && store.Progress.File != "" {
		if store.Progress.entryWrites() {
			batch.Entries = map[string]documentdb.EntryChanges{progressDocument: {Set: entryValues(updates)}}
		} else {
			batch.Documents[filepath.Base(store.Progress.File)] = mergedProgress(*store.Progress.Values, updates)
		}
	}
	if len(batch.Documents) == 0 && len(batch.Entries) == 0 {
		return nil
	}
	return store.Progress.Database.Commit(context.Background(), batch)
}

func (store CommitStore) commitFiles(updates map[string]catalog.PlaybackState, applied int, writes []viewingWrite) error {
	if applied > 0 {
		progress := viewingWrite{store.Progress.File, *store.Progress.Values, mergedProgress(*store.Progress.Values, updates), store.Progress.Persist}
		writes = append([]viewingWrite{progress}, writes...)
	}
	done := make([]viewingWrite, 0, len(writes))
	for _, write := range writes {
		if write.file == "" || write.persist == nil {
			continue
		}
		if err := write.persist(write.file, write.new); err != nil {
			return rollbackViewingWrites(done, err)
		}
		done = append(done, write)
	}
	return nil
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
