package operations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/auditjournal"
	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/liveevents"
	"github.com/MikeO7/kinosail/packages/maintenance"
	"github.com/MikeO7/kinosail/packages/metadata"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/viewing"
	"github.com/MikeO7/kinosail/packages/watchrooms"
)

func TestBindingOwnsRegistrationAndTaskWiring(t *testing.T) { //nolint:cyclop,funlen // One binding proves deferred task selection, routes, handlers, and error propagation.
	t.Parallel()
	var ownerCalls, scanCalls, metadataCalls, cacheCalls, maintainCalls int
	config := testBindingConfig(t, runtimeSnapshot{})
	config.Version, config.Started, config.Now = "v7", time.Unix(100, 0), func() time.Time { return time.Unix(110, 0) }
	config.Index = catalog.NewIndex(t.Context(), nil, "", time.Hour, func(context.Context) (func(), error) { scanCalls++; return func() {}, nil })
	config.Metadata = func() MetadataRefresh {
		metadataCalls++
		return completeEmptyMetadataRefresh()
	}
	config.Cache = playback.NewHLSCacheControl(t.TempDir(), new(sync.Mutex), func(string) bool { return false }, func() bool { cacheCalls++; return false }, playback.HLSRecipePolicy{})
	config.Maintenance = maintenance.New(t.Context(), time.Hour, 1, maintenance.Dependencies{
		PruneCache: func(int64) (int64, error) { maintainCalls++; return 0, nil }, PruneMetadata: func() error { return nil },
	})
	binding := NewBinding(config)
	mux := http.NewServeMux()
	RegisterWeb(mux, Web{
		Owner: func(next http.Handler) http.Handler { ownerCalls++; return next }, WriteError: func(http.ResponseWriter, *http.Request, error, int) {},
		Tasks: binding.Tasks(), Metrics: binding.Metrics, Diagnostics: binding.DiagnosticReport,
	})
	if ownerCalls != 5 || scanCalls != 1 || metadataCalls != 0 || cacheCalls != 0 || maintainCalls != 0 {
		t.Fatalf("registration effects owner=%d scan=%d metadata=%d cache=%d maintain=%d", ownerCalls, scanCalls, metadataCalls, cacheCalls, maintainCalls)
	}
	for _, path := range []string{"/settings/tasks/scan", "/settings/tasks/metadata", "/settings/tasks/maintain"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
		if response.Code != http.StatusSeeOther {
			t.Fatalf("POST %s = %d", path, response.Code)
		}
	}
	for _, path := range []string{"/settings/metrics", "/settings/diagnostics.json"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, response.Code)
		}
	}
	if err := binding.Tasks().ClearCache(t.Context()); err != nil {
		t.Fatal(err)
	}
	if scanCalls != 2 || metadataCalls != 1 || cacheCalls != 1 || maintainCalls != 1 {
		t.Fatalf("task calls scan=%d metadata=%d cache=%d maintain=%d", scanCalls, metadataCalls, cacheCalls, maintainCalls)
	}
}

func TestBindingPreservesTaskErrorsWithoutLaterSideEffects(t *testing.T) { //nolint:cyclop // One matrix proves each task stops at its own error boundary.
	t.Parallel()
	want := errors.New("failed")
	var scanCalls, metadataCalls, cacheCalls, maintainCalls int
	index := catalog.NewIndex(t.Context(), nil, "", time.Hour, func(context.Context) (func(), error) { scanCalls++; return nil, want })
	binding := NewBinding(BindingConfig{
		Index:    index,
		Metadata: func() MetadataRefresh { metadataCalls++; return MetadataRefresh{} },
		Cache:    playback.NewHLSCacheControl(t.TempDir(), new(sync.Mutex), func(string) bool { return false }, func() bool { cacheCalls++; return true }, playback.HLSRecipePolicy{}),
		Maintenance: maintenance.New(t.Context(), time.Hour, 1, maintenance.Dependencies{
			PruneCache: func(int64) (int64, error) { maintainCalls++; return 0, want }, PruneMetadata: func() error { maintainCalls++; return nil },
		}),
	})
	tasks := binding.Tasks()
	if err := tasks.Scan(t.Context()); !errors.Is(err, want) || scanCalls != 2 || metadataCalls != 0 || cacheCalls != 0 || maintainCalls != 0 {
		t.Fatalf("scan err=%v effects=%d,%d,%d,%d", err, scanCalls, metadataCalls, cacheCalls, maintainCalls)
	}
	if err := tasks.ClearCache(t.Context()); err == nil || err.Error() != "transcodes are currently active" || cacheCalls != 1 || maintainCalls != 0 {
		t.Fatalf("cache err=%v effects=%d,%d", err, cacheCalls, maintainCalls)
	}
	if err := tasks.Maintain(t.Context()); !errors.Is(err, want) || maintainCalls != 2 {
		t.Fatalf("maintain err=%v effects=%d", err, maintainCalls)
	}
	if err := tasks.Metadata(t.Context()); err == nil || err.Error() != "metadata provider is not configured" || metadataCalls != 1 {
		t.Fatalf("metadata err=%v effects=%d", err, metadataCalls)
	}
}

func TestBindingProjectsEveryMetric(t *testing.T) { //nolint:cyclop,funlen // Exact field checks protect the complete projection.
	t.Parallel()
	snapshot := fullRuntimeSnapshot()
	metrics := projectMetrics(snapshot)
	want := Metrics{
		Healthy: true, LibraryMonitoring: true, LibraryItems: 1, Sessions: 2, TranscodeCacheBytes: 3, HTTPRequests: 4, HTTPErrors: 5, HTTPPanics: 6, ActivityFailures: 7,
		WorkloadCapacity: 8, WorkloadBackgroundCapacity: 9, ActivePlayback: 10, ActiveBackground: 11, WaitingPlayback: 12, WaitingBackground: 13,
		NotificationQueueDepth: 14, NotificationDropped: 15, ViewingSyncActive: 16, ViewingSyncDeferred: 17,
		LiveEventSubscribers: 18, LiveEventsPublished: 19, LiveEventReconnects: 20, LiveEventRejected: 21, LiveEventSlowDrops: 22,
		WatchRoomConnections: 23, WatchRoomJoins: 24, WatchRoomReconnects: 25, WatchRoomSlowDrops: 26, WatchRoomDriftObservations: 27, WatchRoomDriftSeconds: 28.5,
	}
	if !reflect.DeepEqual(metrics, want) {
		t.Fatalf("metrics = %+v", metrics)
	}
	response := httptest.NewRecorder()
	binding := NewBinding(testBindingConfig(t, snapshot))
	binding.MetricsHandler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("metric response = %d %q", response.Code, response.Body.String())
	}
	snapshot.ScanError = errors.New("scan")
	if projectMetrics(snapshot).Healthy {
		t.Fatal("scan failure reported healthy")
	}
}

func TestBindingProjectsDiagnosticsAtRequestTime(t *testing.T) { //nolint:cyclop // One report protects identity, health, errors, and settings.
	t.Parallel()
	started := time.Date(2026, time.September, 4, 12, 0, 0, 0, time.UTC)
	snapshot := fullRuntimeSnapshot()
	snapshot.CacheError = errors.New("cache")
	report := projectDiagnosticReport("v8", started, started.Add(90*time.Second), snapshot)
	want := DiagnosticReport{
		Version: "v8", Generated: "2026-09-04T12:01:30Z", UptimeSeconds: 90, LastScan: "2026-09-04T11:00:00Z",
		PlaybackMode: "direct", Transcoder: "fast", ScanFrequency: "hourly", LibraryMonitoring: "watching",
		Healthy: false, ScanError: false, LibraryItems: 1, Sessions: 2, TranscodeCacheBytes: 3,
		HTTPRequests: 4, HTTPErrors: 5, HTTPPanics: 6, ActivityFailures: 7, ActivityHealthy: true,
	}
	if !reflect.DeepEqual(report, want) {
		t.Fatalf("diagnostic = %+v", report)
	}
	response := httptest.NewRecorder()
	config := testBindingConfig(t, snapshot)
	config.Version, config.Started, config.Now = "v8", started, func() time.Time { return started.Add(90 * time.Second) }
	binding := NewBinding(config)
	binding.DiagnosticsHandler().ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Disposition") == "" {
		t.Fatalf("diagnostic response = %d %v", response.Code, response.Header())
	}
	snapshot.CacheError, snapshot.ScanError = nil, errors.New("scan")
	if report := projectDiagnosticReport("v8", started, started, snapshot); report.Healthy || !report.ScanError {
		t.Fatalf("scan report = %+v", report)
	}
}

func completeEmptyMetadataRefresh() MetadataRefresh {
	return MetadataRefresh{
		Available: true,
		Record:    func(string) (metadata.Record, bool) { return metadata.Record{}, false },
		Resolve:   func(context.Context, library.Item) (metadata.Result, error) { return metadata.Result{}, nil },
		ResolveEpisode: func(context.Context, library.Item, metadata.Record) (metadata.Result, error) {
			return metadata.Result{}, nil
		},
		Download: func(context.Context, metadata.Result) (metadata.Record, error) { return metadata.Record{}, nil },
		Store:    func(map[string]metadata.Record) error { return nil }, RefreshLibrary: func(context.Context) error { return nil },
	}
}

func fullRuntimeSnapshot() runtimeSnapshot {
	return runtimeSnapshot{
		LastScan: time.Date(2026, time.September, 4, 11, 0, 0, 0, time.UTC), PlaybackMode: "direct", Transcoder: "fast", ScanFrequency: "hourly", LibraryMonitoring: true,
		LibraryItems: 1, Sessions: 2, TranscodeCacheBytes: 3, HTTPRequests: 4, HTTPErrors: 5, HTTPPanics: 6,
		Activity:          auditjournal.Status{Healthy: true, WriteFailures: 7, NotificationQueued: 14, NotificationDrops: 15},
		Workload:          WorkloadSnapshot{Capacity: 8, BackgroundCapacity: 9, ActivePlayback: 10, ActiveBackground: 11, WaitingPlayback: 12, WaitingBackground: 13},
		ViewingSyncActive: 16, ViewingSyncDeferred: 17,
		LiveEvents: liveevents.Metrics{Subscribers: 18, Published: 19, Reconnects: 20, Rejected: 21, SlowDrops: 22},
		WatchRooms: watchrooms.Metrics{Connections: 23, Joins: 24, Reconnects: 25, SlowDrops: 26, DriftObservations: 27, DriftSeconds: 28.5},
	}
}

func testBindingConfig(t *testing.T, snapshot runtimeSnapshot) BindingConfig {
	t.Helper()
	return BindingConfig{
		Index: catalog.NewMemoryIndex(make([]library.Item, snapshot.LibraryItems), true),
		Cache: playback.NewHLSCacheControl(t.TempDir(), new(sync.Mutex), func(string) bool { return false }, func() bool { return false }, playback.HLSRecipePolicy{}),
		Maintenance: maintenance.New(t.Context(), time.Hour, 1, maintenance.Dependencies{
			PruneCache: func(int64) (int64, error) { return 0, nil }, PruneMetadata: func() error { return nil },
		}),
		Activity: auditjournal.NewHTTPTracker(nil, auditjournal.HTTPConfig{}), Viewing: new(viewing.Manager), Events: liveevents.New(), Rooms: watchrooms.New(time.Hour),
		Metadata: completeEmptyMetadataRefresh,
		Sessions: func() int { return snapshot.Sessions },
		PlaybackSettings: func() (string, string, string) {
			return snapshot.PlaybackMode, snapshot.Transcoder, snapshot.ScanFrequency
		},
		HTTPActivity: func() (uint64, uint64, uint64) {
			return snapshot.HTTPRequests, snapshot.HTTPErrors, snapshot.HTTPPanics
		},
		Workload: func() WorkloadSnapshot { return snapshot.Workload },
	}
}
