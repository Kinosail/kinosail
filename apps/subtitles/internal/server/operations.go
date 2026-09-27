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

func metadataRefresh(store *metadataStore, index *libraryIndex) sharedoperations.MetadataRefresh {
	items, _ := index.Snapshot()
	return sharedoperations.MetadataRefresh{
		Items: items, Configured: store.configured(), Available: store.available(),
		Store: store.setAll, RefreshLibrary: index.Refresh,
		Download: store.downloadMetadataImages, ResolveEpisode: store.resolveEpisodeRecord, Resolve: store.resolveRecord,
		Record: func(id string) (metadata.Record, bool) {
			store.mu.RLock()
			defer store.mu.RUnlock()
			record, found := store.records[id]
			return record, found
		},
	}
}

func operationDiagnostics(settings *settingsStore, index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore) sharedoperations.Binding {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Activity: audit.HTTPTracker, Cache: hls.cacheOps, Index: index.Index, Now: time.Now, Started: applicationStarted, Version: ApplicationVersion,
		HTTPActivity: func() (uint64, uint64, uint64) {
			return audit.requests.Load(), audit.requestErrors.Load(), audit.panics.Load()
		},
		Failures: audit.failures.Recent,
		PlaybackSettings: func() (string, string, string) {
			return settings.playbackMode(), settings.transcoding().Name, settings.scanFrequency()
		}, Sessions: profiles.activeSessions,
	})
}

func buildDiagnosticReport(settings *settingsStore, index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore) diagnosticReport {
	return operationDiagnostics(settings, index, profiles, hls, audit).DiagnosticReport()
}

func diagnostics(settings *settingsStore, index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore) http.HandlerFunc {
	return operationDiagnostics(settings, index, profiles, hls, audit).DiagnosticsHandler()
}

func operationMetrics(index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore, imports *viewingImportManager, events *liveEventHub, rooms *watchRoomAdapter) sharedoperations.Binding {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Rooms: rooms.rooms, Events: events, Viewing: imports.previewer, Activity: audit.HTTPTracker, Cache: hls.cacheOps, Index: index.Index,
		Workload: func() sharedoperations.WorkloadSnapshot {
			value := hls.workloads.Metrics()
			return sharedoperations.WorkloadSnapshot{Capacity: value.Capacity, BackgroundCapacity: value.BackgroundCapacity, ActivePlayback: value.ActivePlayback, ActiveBackground: value.ActiveBackground, WaitingPlayback: value.WaitingPlayback, WaitingBackground: value.WaitingBackground}
		},
		HTTPActivity: func() (uint64, uint64, uint64) {
			return audit.requests.Load(), audit.requestErrors.Load(), audit.panics.Load()
		}, Sessions: profiles.activeSessions,
	})
}

func metrics(index *libraryIndex, profiles *profileStore, hls *hlsManager, audit *auditStore, imports *viewingImportManager, events *liveEventHub, rooms *watchRoomAdapter) http.HandlerFunc {
	return operationMetrics(index, profiles, hls, audit, imports, events, rooms).MetricsHandler()
}

func (store *metadataStore) refreshMissing(ctx context.Context, index *libraryIndex) error {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{Metadata: func() sharedoperations.MetadataRefresh { return metadataRefresh(store, index) }}).RefreshMetadata(ctx)
}

func operationTasks(index *libraryIndex, hls *hlsManager, metadata *metadataStore, maintenance *maintenanceManager) sharedoperations.Tasks {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Maintenance: maintenance, Cache: hls.cacheOps, Index: index.Index,
		Metadata: func() sharedoperations.MetadataRefresh { return metadataRefresh(metadata, index) },
	}).Tasks()
}

func registerOperations(mux *http.ServeMux, auth *authentication, settings *settingsStore, index *libraryIndex, hls *hlsManager, metadata *metadataStore, maintenance *maintenanceManager, imports *viewingImportManager, events *liveEventHub, rooms *watchRoomAdapter) {
	sharedoperations.RegisterWeb(mux, sharedoperations.Web{
		Diagnostics: operationDiagnostics(settings, index, auth.profiles, hls, auth.audit).DiagnosticReport,
		Metrics:     operationMetrics(index, auth.profiles, hls, auth.audit, imports, events, rooms).Metrics, Tasks: operationTasks(index, hls, metadata, maintenance),
		WriteError: func(writer http.ResponseWriter, request *http.Request, err error, status int) {
			localizedError(writer, request, err.Error(), status)
		}, Owner: auth.owner,
	})
}
