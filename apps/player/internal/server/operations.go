package server

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/metadata"
	sharedoperations "github.com/MikeO7/kinosail/packages/operations"
)

type diagnosticReport = sharedoperations.DiagnosticReport

var (
	ApplicationVersion = "dev"
	applicationStarted = time.Now()
)

func registerOperations(mux *http.ServeMux, auth *authentication, settings *settingsStore, index *libraryIndex, hls *hlsManager, metadata *metadataStore, maintenance *maintenanceManager, imports *viewingImportManager, events *liveEventHub, rooms *watchRoomAdapter) {
	sharedoperations.RegisterWeb(mux, sharedoperations.Web{
		Owner: auth.owner, WriteError: func(writer http.ResponseWriter, request *http.Request, err error, status int) {
			localizedError(writer, request, err.Error(), status)
		},
		Tasks: operationTasks(index, hls, metadata, maintenance), Metrics: operationMetrics(index, auth.profiles, hls, auth.audit, imports, events, rooms).Metrics,
		Diagnostics: operationDiagnostics(settings, index, auth.profiles, hls, auth.audit).DiagnosticReport,
	})
}

func operationTasks(index *libraryIndex, hls *hlsManager, metadata *metadataStore, maintenance *maintenanceManager) sharedoperations.Tasks {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Index: index.Index, Cache: hls.cacheOps, Maintenance: maintenance.Manager,
		Metadata: func() sharedoperations.MetadataRefresh { return metadataRefresh(metadata, index) },
	}).Tasks()
}

func (store *metadataStore) refreshMissing(ctx context.Context, index *libraryIndex) error {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{Metadata: func() sharedoperations.MetadataRefresh { return metadataRefresh(store, index) }}).RefreshMetadata(ctx)
}

func metadataRefresh(store *metadataStore, index *libraryIndex) sharedoperations.MetadataRefresh {
	items, _ := index.Snapshot()
	return sharedoperations.MetadataRefresh{
		Available: store.available(), Configured: store.configured(), Items: items,
		Record: func(id string) (metadata.Record, bool) {
			store.mu.RLock()
			defer store.mu.RUnlock()
			record, found := store.records[id]
			return record, found
		},
		Resolve: store.resolveRecord, ResolveEpisode: store.resolveEpisodeRecord, Download: store.downloadMetadataImages,
		Store: store.setAll, RefreshLibrary: index.Refresh,
	}
}

func operationMetrics(index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore, imports *viewingImportManager, events *liveEventHub, rooms *watchRoomAdapter) sharedoperations.Binding {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Index: index.Index, Cache: hls.cacheOps, Activity: audit.HTTPTracker, Viewing: imports.previewer, Events: events, Rooms: rooms.rooms,
		Sessions: profiles.activeSessions, HTTPActivity: func() (uint64, uint64, uint64) {
			return audit.requests.Load(), audit.requestErrors.Load(), audit.panics.Load()
		},
		Workload: func() sharedoperations.WorkloadSnapshot {
			value := hls.workloads.Metrics()
			return sharedoperations.WorkloadSnapshot{Capacity: value.Capacity, BackgroundCapacity: value.BackgroundCapacity, ActivePlayback: value.ActivePlayback, ActiveBackground: value.ActiveBackground, WaitingPlayback: value.WaitingPlayback, WaitingBackground: value.WaitingBackground}
		},
	})
}

func metrics(index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore, imports *viewingImportManager, events *liveEventHub, rooms *watchRoomAdapter) http.HandlerFunc {
	return operationMetrics(index, profiles, hls, audit, imports, events, rooms).MetricsHandler()
}

func operationDiagnostics(settings *settingsStore, index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore) sharedoperations.Binding {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Version: ApplicationVersion, Started: applicationStarted, Now: time.Now, Index: index.Index, Cache: hls.cacheOps, Activity: audit.HTTPTracker,
		Sessions: profiles.activeSessions, PlaybackSettings: func() (string, string, string) {
			return settings.playbackMode(), settings.transcoding().Name, settings.scanFrequency()
		},
		HTTPActivity: func() (uint64, uint64, uint64) {
			return audit.requests.Load(), audit.requestErrors.Load(), audit.panics.Load()
		},
	})
}

func diagnostics(settings *settingsStore, index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore) http.HandlerFunc {
	return operationDiagnostics(settings, index, profiles, hls, audit).DiagnosticsHandler()
}

func buildDiagnosticReport(settings *settingsStore, index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore) diagnosticReport {
	return operationDiagnostics(settings, index, profiles, hls, audit).DiagnosticReport()
}
