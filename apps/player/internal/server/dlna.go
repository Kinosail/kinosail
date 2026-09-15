package server

import (
	"context"
	"net/http"

	"github.com/MikeO7/kinosail/packages/dlna"
)

func registerDLNA(mux *http.ServeMux, ctx context.Context, base string, settings *settingsStore, index *libraryIndex) {
	_ = dlna.Register(mux, ctx, dlna.Config{
		Base: base,
		Settings: dlna.SettingsFuncs{
			TokenFunc: settings.dlnaToken, ServerNameFunc: settings.serverName,
		},
		Library: dlna.LibraryFuncs{
			SnapshotFunc: index.Snapshot, FindFunc: index.Find, SafeFunc: index.Safe,
		},
		NotFound: localizedNotFound,
	})
}
