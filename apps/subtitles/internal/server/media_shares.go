package server

import (
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/mediashares"
)

type mediaShareStore = mediashares.Store

func newMediaShares(dataDir string, stateDB *database.Store, index *libraryIndex) *mediaShareStore {
	return mediashares.New(dataDir, mediashares.Dependencies{
		Load:    func(path string, target any) (bool, error) { return loadState(stateDB, path, target) },
		Persist: statePersistence(stateDB), Find: index.Find, Snapshot: index.Snapshot,
	})
}
