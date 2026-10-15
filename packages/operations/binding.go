package operations

import (
	"context"
	"net/http"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/liveevents"
	"github.com/MikeO7/kinosail/packages/maintenance"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/viewing"
	"github.com/MikeO7/kinosail/packages/watchrooms"
)

type BindingConfig struct {
	Version          string
	Started          time.Time
	Now              func() time.Time
	Index            *catalog.Index
	Cache            playback.HLSCacheControl
	Maintenance      *maintenance.Manager
	Activity         *auditjournal.HTTPTracker
	Viewing          *viewing.Manager
	Events           *liveevents.Hub
	Rooms            *watchrooms.Rooms
	Metadata         func() MetadataRefresh
	Sessions         func() int
	PlaybackSettings func() (string, string, string)
	HTTPActivity     func() (uint64, uint64, uint64)
	Failures         func() []FailureEvent
	Workload         func() WorkloadSnapshot
}

type Binding struct{ config BindingConfig }

func NewBinding(config BindingConfig) Binding { return Binding{config} }

func (binding Binding) Tasks() Tasks {
	return Tasks{
		Scan:       binding.config.Index.Refresh,
		Metadata:   binding.RefreshMetadata,
		ClearCache: func(context.Context) error { return binding.clearCache() },
		Maintain:   func(context.Context) error { return binding.config.Maintenance.Run() },
	}
}

func (binding Binding) RefreshMetadata(ctx context.Context) error {
	return RefreshMetadata(ctx, binding.config.Metadata())
}

func (binding Binding) Metrics() Metrics { return projectMetrics(binding.metricSnapshot()) }

func (binding Binding) MetricsHandler() http.HandlerFunc { return MetricsHandler(binding.Metrics) }

func (binding Binding) DiagnosticReport() DiagnosticReport {
	report := projectDiagnosticReport(binding.config.Version, binding.config.Started, binding.config.Now(), binding.diagnosticSnapshot())
	if binding.config.Failures != nil {
		report.RecentFailures = binding.config.Failures()
	}
	return report
}

func (binding Binding) DiagnosticsHandler() http.HandlerFunc {
	return DiagnosticsHandler(binding.DiagnosticReport)
}

func (binding Binding) metricSnapshot() runtimeSnapshot {
	items, _, scanErr := binding.config.Index.Status()
	monitoring, _ := binding.config.Index.Monitoring()
	cache, _ := binding.cacheStats()
	workload, live, rooms, activity := binding.workload(), binding.config.Events.Metrics(), binding.config.Rooms.Metrics(), binding.config.Activity.Status()
	syncActive, syncDeferred := binding.config.Viewing.Metrics()
	sessions := binding.config.Sessions()
	requests, requestErrors, panics := binding.config.HTTPActivity()
	return runtimeSnapshot{
		LibraryMonitoring: monitoring, ScanError: scanErr, LibraryItems: items, Sessions: sessions, TranscodeCacheBytes: cache,
		HTTPRequests: requests, HTTPErrors: requestErrors, HTTPPanics: panics, Activity: activity, Workload: workload,
		ViewingSyncActive: syncActive, ViewingSyncDeferred: syncDeferred, LiveEvents: live, WatchRooms: rooms,
	}
}

func (binding Binding) diagnosticSnapshot() runtimeSnapshot {
	items, scanned, scanErr := binding.config.Index.Status()
	monitoring, _ := binding.config.Index.Monitoring()
	cache, cacheErr := binding.cacheStats()
	activity := binding.config.Activity.Status()
	playback, transcoder, frequency := binding.playbackSettings()
	sessions := binding.config.Sessions()
	requests, requestErrors, panics := binding.config.HTTPActivity()
	return runtimeSnapshot{
		LastScan: scanned, PlaybackMode: playback, Transcoder: transcoder, ScanFrequency: frequency, LibraryMonitoring: monitoring,
		ScanError: scanErr, CacheError: cacheErr, LibraryItems: items, Sessions: sessions, TranscodeCacheBytes: cache,
		HTTPRequests: requests, HTTPErrors: requestErrors, HTTPPanics: panics, Activity: activity,
	}
}

func (binding Binding) cacheStats() (int64, error) {
	if binding.config.Cache.Lock == nil {
		return 0, nil
	}
	return binding.config.Cache.Stats()
}

func (binding Binding) clearCache() error {
	if binding.config.Cache.Lock == nil {
		return nil
	}
	return binding.config.Cache.Clear()
}

func (binding Binding) workload() WorkloadSnapshot {
	if binding.config.Workload == nil {
		return WorkloadSnapshot{}
	}
	return binding.config.Workload()
}

func (binding Binding) playbackSettings() (string, string, string) {
	if binding.config.PlaybackSettings == nil {
		return "", "", ""
	}
	return binding.config.PlaybackSettings()
}
