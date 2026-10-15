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

func operationDiagnostics(settings *settingsStore, index *libraryIndex, profiles *profileStore, audit *auditStore) sharedoperations.Binding {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Activity: audit.HTTPTracker, Index: index.Index, Now: time.Now, Started: applicationStarted, Version: ApplicationVersion,
		HTTPActivity: func() (uint64, uint64, uint64) {
			return audit.requests.Load(), audit.requestErrors.Load(), audit.panics.Load()
		},
		Failures: audit.failures.Recent,
		PlaybackSettings: func() (string, string, string) {
			return "", "", settings.scanFrequency()
		}, Sessions: profiles.activeSessions,
	})
}

func buildDiagnosticReport(settings *settingsStore, index *libraryIndex, profiles *profileStore, audit *auditStore) diagnosticReport {
	return operationDiagnostics(settings, index, profiles, audit).DiagnosticReport()
}

func diagnostics(settings *settingsStore, index *libraryIndex, profiles *profileStore, audit *auditStore) http.HandlerFunc {
	return operationDiagnostics(settings, index, profiles, audit).DiagnosticsHandler()
}

func operationMetrics(index *libraryIndex, profiles *profileStore, audit *auditStore, events *liveEventHub) sharedoperations.Binding {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Events: events, Activity: audit.HTTPTracker, Index: index.Index,
		HTTPActivity: func() (uint64, uint64, uint64) {
			return audit.requests.Load(), audit.requestErrors.Load(), audit.panics.Load()
		}, Sessions: profiles.activeSessions,
	})
}

func metrics(index *libraryIndex, profiles *profileStore, audit *auditStore, events *liveEventHub) http.HandlerFunc {
	return operationMetrics(index, profiles, audit, events).MetricsHandler()
}

func (store *metadataStore) refreshMissing(ctx context.Context, index *libraryIndex) error {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{Metadata: func() sharedoperations.MetadataRefresh { return metadataRefresh(store, index) }}).RefreshMetadata(ctx)
}

func operationTasks(index *libraryIndex, metadata *metadataStore, maintenance *maintenanceManager) sharedoperations.Tasks {
	return sharedoperations.NewBinding(sharedoperations.BindingConfig{
		Maintenance: maintenance, Index: index.Index,
		Metadata: func() sharedoperations.MetadataRefresh { return metadataRefresh(metadata, index) },
	}).Tasks()
}

func registerOperations(mux *http.ServeMux, auth *authentication, settings *settingsStore, index *libraryIndex, metadata *metadataStore, maintenance *maintenanceManager, events *liveEventHub) {
	sharedoperations.RegisterWeb(mux, sharedoperations.Web{
		Diagnostics: operationDiagnostics(settings, index, auth.profiles, auth.audit).DiagnosticReport,
		Metrics:     operationMetrics(index, auth.profiles, auth.audit, events).Metrics, Tasks: operationTasks(index, metadata, maintenance),
		WriteError: func(writer http.ResponseWriter, request *http.Request, err error, status int) {
			localizedError(writer, request, err.Error(), status)
		}, Owner: auth.owner,
	})
}
