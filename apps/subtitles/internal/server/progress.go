package server

import (
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

type (
	progressStore = catalog.RequestProgressStore
	playbackState = catalog.PlaybackState
)

type (
	resumeItem   = catalog.ResumeItem
	playbackView = catalog.PlaybackView
)

func newProgressStore(dataDir string, databases ...*database.Store) *progressStore {
	stateDB := configuredDatabase(databases)
	return catalogapi.NewProgressStore(dataDir, stateDB, func(file string, target any) (bool, error) { return loadState(stateDB, file, target) }, statePersistence(stateDB), currentViewer)
}
