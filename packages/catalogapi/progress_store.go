package catalogapi

import (
	"net/http"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/documentdb"
	"github.com/MikeO7/kinosail/packages/identitycore"
	"github.com/MikeO7/kinosail/packages/library"
)

type progressIndex interface {
	Snapshot() ([]library.Item, error)
}

// NewProgressStore binds the canonical Viewer and public item projection to durable progress.
func NewProgressStore(dataDir string, database *documentdb.Store, load func(string, any) (bool, error), persist func(string, any) error, viewer func(*http.Request) identitycore.Profile) *catalog.RequestProgressStore {
	return catalog.NewRequestProgressStore(dataDir, database, load, persist, func(request *http.Request) catalog.ProgressViewer {
		profile := viewer(request)
		return catalog.ProgressViewer{ID: profile.ID, Owner: profile.Owner}
	}, func(request *http.Request, item library.Item, state catalog.PlaybackState) any {
		return ProjectViewerItem(item, state, viewer(request))
	})
}

// ProjectViewerItem applies one Viewer's public stream and download permissions.
func ProjectViewerItem(item library.Item, state catalog.PlaybackState, viewer identitycore.Profile) ClientItem {
	access := ItemAccess{Stream: viewer.Permits("stream", true), Download: viewer.Permits("download", viewer.Owner || viewer.Downloads)}
	return ProjectItem(item, state, access)
}

// RecentAdminProgress projects recent activity across all Viewer profiles.
func RecentAdminProgress(store *catalog.RequestProgressStore, index progressIndex, profiles []identitycore.Profile) []catalog.PlaybackView {
	items, _ := index.Snapshot()
	names := make(map[string]string, len(profiles))
	for _, profile := range profiles {
		names[profile.ID] = profile.Name
	}
	return store.RecentAdmin(items, names)
}
