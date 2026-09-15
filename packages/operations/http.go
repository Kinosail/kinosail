package operations

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ErrorWriter translates an operation failure to an application HTTP response.
type ErrorWriter func(http.ResponseWriter, *http.Request, error, int)

// Tasks contains the complete set of owner-run server tasks.
type Tasks struct {
	Scan, Metadata, ClearCache, Maintain func(context.Context) error
}

// Web configures the owner-only operational web routes.
type Web struct {
	Owner       func(http.Handler) http.Handler
	WriteError  ErrorWriter
	Tasks       Tasks
	Metrics     func() Metrics
	Diagnostics func() DiagnosticReport
}

// RegisterWeb registers owner-only task, metrics, and diagnostic routes.
func RegisterWeb(mux *http.ServeMux, config Web) {
	mux.Handle("GET /settings/metrics", config.Owner(MetricsHandler(config.Metrics)))
	mux.Handle("GET /settings/diagnostics.json", config.Owner(DiagnosticsHandler(config.Diagnostics)))
	mux.Handle("POST /settings/tasks/scan", config.Owner(TaskHandler(config.Tasks.Scan, config.WriteError)))
	mux.Handle("POST /settings/tasks/metadata", config.Owner(TaskHandler(config.Tasks.Metadata, config.WriteError)))
	mux.Handle("POST /settings/tasks/maintain", config.Owner(TaskHandler(config.Tasks.Maintain, config.WriteError)))
}

// TaskHandler runs one web task and returns to settings after success.
func TaskHandler(run func(context.Context) error, writeError ErrorWriter) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if err := run(request.Context()); err != nil {
			writeError(writer, request, err, http.StatusBadGateway)
			return
		}
		http.Redirect(writer, request, "/settings", http.StatusSeeOther)
	}
}

// APITaskHandler selects and runs one versioned server task.
func APITaskHandler(tasks Tasks, writeError ErrorWriter, notFound http.HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var run func(context.Context) error
		switch request.PathValue("task") {
		case "scan":
			run = tasks.Scan
		case "metadata":
			run = tasks.Metadata
		case "clear-cache":
			run = tasks.ClearCache
		case "maintain":
			run = tasks.Maintain
		default:
			notFound(writer, request)
			return
		}
		if err := run(request.Context()); err != nil {
			writeError(writer, request, err, http.StatusBadGateway)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}
}

// MetricsHandler serves one current metrics snapshot.
func MetricsHandler(snapshot func() Metrics) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		value := snapshot()
		writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = fmt.Fprintf(writer, "kinosail_health %d\nkinosail_library_items %d\nkinosail_library_monitoring %d\nkinosail_sessions %d\nkinosail_transcode_cache_bytes %d\nkinosail_http_requests_total %d\nkinosail_http_errors_total %d\nkinosail_http_panics_total %d\nkinosail_audit_write_failures_total %d\nkinosail_workload_capacity %d\nkinosail_workload_background_capacity %d\nkinosail_workload_active{class=\"playback\"} %d\nkinosail_workload_active{class=\"background\"} %d\nkinosail_workload_waiting{class=\"playback\"} %d\nkinosail_workload_waiting{class=\"background\"} %d\n", boolMetric(value.Healthy), value.LibraryItems, boolMetric(value.LibraryMonitoring), value.Sessions, value.TranscodeCacheBytes, value.HTTPRequests, value.HTTPErrors, value.HTTPPanics, value.ActivityFailures, value.WorkloadCapacity, value.WorkloadBackgroundCapacity, value.ActivePlayback, value.ActiveBackground, value.WaitingPlayback, value.WaitingBackground)
		_, _ = fmt.Fprintf(writer, "kinosail_notification_queue_depth %d\nkinosail_notification_dropped_total %d\nkinosail_viewing_sync_active %d\nkinosail_viewing_sync_deferred_total %d\n", value.NotificationQueueDepth, value.NotificationDropped, value.ViewingSyncActive, value.ViewingSyncDeferred)
		_, _ = fmt.Fprintf(writer, "kinosail_live_event_subscribers %d\nkinosail_live_events_published_total %d\nkinosail_live_event_reconnects_total %d\nkinosail_live_event_rejected_total %d\nkinosail_live_event_slow_drops_total %d\n", value.LiveEventSubscribers, value.LiveEventsPublished, value.LiveEventReconnects, value.LiveEventRejected, value.LiveEventSlowDrops)
		_, _ = fmt.Fprintf(writer, "kinosail_watch_room_connections %d\nkinosail_watch_room_joins_total %d\nkinosail_watch_room_reconnects_total %d\nkinosail_watch_room_slow_drops_total %d\nkinosail_watch_room_drift_observations_total %d\nkinosail_watch_room_drift_seconds %g\n", value.WatchRoomConnections, value.WatchRoomJoins, value.WatchRoomReconnects, value.WatchRoomSlowDrops, value.WatchRoomDriftObservations, value.WatchRoomDriftSeconds)
	}
}

// DiagnosticsHandler serves one downloadable safe diagnostic report.
func DiagnosticsHandler(report func() DiagnosticReport) http.HandlerFunc {
	return func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		writer.Header().Set("Content-Disposition", `attachment; filename="kinosail-diagnostics.json"`)
		_ = json.NewEncoder(writer).Encode(report())
	}
}

func boolMetric(value bool) int {
	if value {
		return 1
	}
	return 0
}
