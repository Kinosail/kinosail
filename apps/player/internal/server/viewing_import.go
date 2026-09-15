package server

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/viewing"
)

type (
	viewingImportInput   = viewing.Input
	viewingImportItem    = viewing.Item
	viewingImportSummary = viewing.Summary
	viewingImportPreview = viewing.Preview
	viewingSyncView      = viewing.SyncView
)

type viewingImportManager struct {
	previewer *viewing.Manager
	profiles  *profileStore
}

func newViewingImportManager(ctx context.Context, dataDir string, index *libraryIndex, progress *progressStore, lists *listStore, profiles *profileStore) *viewingImportManager {
	manager := &viewingImportManager{profiles: profiles}
	progressStorage := viewing.NewProgressStorage(progress.Storage(), progress.Database())
	committer := viewing.CommitStore{Progress: progressStorage, Lists: lists.storage(), ListMutex: &lists.mu}
	manager.previewer = viewing.NewManager(ctx, dataDir, viewing.Config{
		Client: localIntegrationHTTPClient(30 * time.Second), Persist: saveJSON, Snapshot: index.Snapshot,
		Profile: func(id string) (viewing.Profile, bool) {
			profile, found := profiles.byID(id)
			return viewing.Profile{ID: profile.ID, Name: profile.Name, Owner: profile.Owner}, found
		},
		Progress: func(profile viewing.Profile, id string) catalog.PlaybackState {
			return progress.GetFor(profile.ID, profile.Owner, id)
		},
		ListAdditions: func(profile viewing.Profile, id string, favorite bool, playlists map[string]int) int {
			return lists.viewingImportable(profile.ID, id, favorite, playlists)
		},
		ImportProgress: func(profile viewing.Profile, changes []viewing.ProgressChange) (int, int, error) {
			storage := progressStorage
			storage.ProfileID, storage.Owner = profile.ID, profile.Owner
			return storage.Import(changes, time.Now())
		},
		Commit: committer.Commit,
	})
	return manager
}

func (manager *viewingImportManager) createSync(id, interval string) (viewingSyncView, error) {
	return manager.previewer.CreateSync(id, interval)
}

func (manager *viewingImportManager) run(ctx context.Context, id string) (viewingSyncView, error) {
	return manager.previewer.RunSync(ctx, id)
}

func (manager *viewingImportManager) list() []viewingSyncView { return manager.previewer.Syncs() }

func (manager *viewingImportManager) remove(id string) error { return manager.previewer.RemoveSync(id) }
