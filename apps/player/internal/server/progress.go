package server

import (
	"time"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/catalogapi"
)

const progressFlushInterval = 5 * time.Second

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

func startProgressCoalescing(progress *progressStore, shutdown *databaseShutdown, managed bool) {
	if managed {
		shutdown.beforeClose(progress.StartWriteCoalescing(progressFlushInterval))
	}
}
