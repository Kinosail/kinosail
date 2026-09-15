package operations

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterWebServesPlayerOperationalContract(t *testing.T) { //nolint:cyclop,funlen // One route test proves the complete shared registry.
	t.Parallel()
	mux := http.NewServeMux()
	var ownerCalls, scanCalls, metadataCalls, maintainCalls int
	RegisterWeb(mux, Web{
		Owner:      func(next http.Handler) http.Handler { ownerCalls++; return next },
		WriteError: func(http.ResponseWriter, *http.Request, error, int) {},
		Tasks: Tasks{
			Scan:     func(context.Context) error { scanCalls++; return nil },
			Metadata: func(context.Context) error { metadataCalls++; return nil },
			Maintain: func(context.Context) error { maintainCalls++; return nil },
		},
		Metrics:     func() Metrics { return Metrics{Healthy: true, LibraryItems: 7} },
		Diagnostics: func() DiagnosticReport { return DiagnosticReport{Version: "test"} },
	})
	if ownerCalls != 5 {
		t.Fatalf("owner adapters = %d", ownerCalls)
	}
	for path, body := range map[string]string{
		"/settings/metrics":          "kinosail_library_items 7",
		"/settings/diagnostics.json": `"version":"test"`,
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), body) {
			t.Fatalf("GET %s = %d %q", path, response.Code, response.Body.String())
		}
	}
	for _, path := range []string{"/settings/tasks/scan", "/settings/tasks/metadata", "/settings/tasks/maintain"} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodPost, path, nil))
		if response.Code != http.StatusSeeOther || response.Header().Get("Location") != "/settings" {
			t.Fatalf("POST %s = %d %v", path, response.Code, response.Header())
		}
	}
	if scanCalls != 1 || metadataCalls != 1 || maintainCalls != 1 {
		t.Fatalf("tasks = %d, %d, %d", scanCalls, metadataCalls, maintainCalls)
	}
}

func TestTaskHandlersTranslateFailuresAndSelections(t *testing.T) { //nolint:cyclop,funlen // One table proves all task names and shared failure behavior.
	t.Parallel()
	failure := errors.New("task failed")
	var errorValue error
	var errorStatus, notFoundCalls int
	writeError := func(_ http.ResponseWriter, _ *http.Request, err error, status int) {
		errorValue, errorStatus = err, status
	}
	failed := httptest.NewRecorder()
	TaskHandler(func(context.Context) error { return failure }, writeError).ServeHTTP(failed, httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/", nil))
	if !errors.Is(errorValue, failure) || errorStatus != http.StatusBadGateway || failed.Code != http.StatusOK {
		t.Fatalf("web failure = %v, %d, %d", errorValue, errorStatus, failed.Code)
	}

	calls := make(map[string]int)
	tasks := Tasks{
		Scan:       func(context.Context) error { calls["scan"]++; return nil },
		Metadata:   func(context.Context) error { calls["metadata"]++; return nil },
		ClearCache: func(context.Context) error { calls["clear-cache"]++; return nil },
		Maintain:   func(context.Context) error { calls["maintain"]++; return nil },
	}
	handler := APITaskHandler(tasks, writeError, func(http.ResponseWriter, *http.Request) { notFoundCalls++ })
	for _, task := range []string{"scan", "metadata", "clear-cache", "maintain"} {
		response := apiTaskRequest(t, handler, task)
		if response.Code != http.StatusNoContent || calls[task] != 1 {
			t.Fatalf("task %q = %d, calls %d", task, response.Code, calls[task])
		}
	}
	apiTaskRequest(t, handler, "unknown")
	if notFoundCalls != 1 {
		t.Fatalf("not found calls = %d", notFoundCalls)
	}
	tasks.Metadata = func(context.Context) error { return failure }
	errorValue, errorStatus = nil, 0
	response := apiTaskRequest(t, APITaskHandler(tasks, writeError, func(http.ResponseWriter, *http.Request) {}), "metadata")
	if !errors.Is(errorValue, failure) || errorStatus != http.StatusBadGateway || response.Code != http.StatusOK {
		t.Fatalf("API failure = %v, %d, %d", errorValue, errorStatus, response.Code)
	}
}

func TestMetricsHandlerWritesEveryPlayerMetric(t *testing.T) { //nolint:funlen // Exact body comparison protects every stable metric name and value.
	t.Parallel()
	value := Metrics{
		Healthy: true, LibraryMonitoring: true, LibraryItems: 1, Sessions: 2, TranscodeCacheBytes: 3, HTTPRequests: 4, HTTPErrors: 5, HTTPPanics: 6, ActivityFailures: 7,
		WorkloadCapacity: 8, WorkloadBackgroundCapacity: 9, ActivePlayback: 10, ActiveBackground: 11, WaitingPlayback: 12, WaitingBackground: 13,
		NotificationQueueDepth: 14, NotificationDropped: 15, ViewingSyncActive: 16, ViewingSyncDeferred: 17,
		LiveEventSubscribers: 18, LiveEventsPublished: 19, LiveEventReconnects: 20, LiveEventRejected: 21, LiveEventSlowDrops: 22,
		WatchRoomConnections: 23, WatchRoomJoins: 24, WatchRoomReconnects: 25, WatchRoomSlowDrops: 26, WatchRoomDriftObservations: 27, WatchRoomDriftSeconds: 28.5,
	}
	response := httptest.NewRecorder()
	MetricsHandler(func() Metrics { return value }).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	want := "kinosail_health 1\nkinosail_library_items 1\nkinosail_library_monitoring 1\nkinosail_sessions 2\nkinosail_transcode_cache_bytes 3\nkinosail_http_requests_total 4\nkinosail_http_errors_total 5\nkinosail_http_panics_total 6\nkinosail_audit_write_failures_total 7\nkinosail_workload_capacity 8\nkinosail_workload_background_capacity 9\nkinosail_workload_active{class=\"playback\"} 10\nkinosail_workload_active{class=\"background\"} 11\nkinosail_workload_waiting{class=\"playback\"} 12\nkinosail_workload_waiting{class=\"background\"} 13\nkinosail_notification_queue_depth 14\nkinosail_notification_dropped_total 15\nkinosail_viewing_sync_active 16\nkinosail_viewing_sync_deferred_total 17\nkinosail_live_event_subscribers 18\nkinosail_live_events_published_total 19\nkinosail_live_event_reconnects_total 20\nkinosail_live_event_rejected_total 21\nkinosail_live_event_slow_drops_total 22\nkinosail_watch_room_connections 23\nkinosail_watch_room_joins_total 24\nkinosail_watch_room_reconnects_total 25\nkinosail_watch_room_slow_drops_total 26\nkinosail_watch_room_drift_observations_total 27\nkinosail_watch_room_drift_seconds 28.5\n"
	if response.Header().Get("Content-Type") != "text/plain; version=0.0.4" || response.Body.String() != want {
		t.Fatalf("metrics headers=%v\n%s", response.Header(), response.Body.String())
	}
	response = httptest.NewRecorder()
	MetricsHandler(func() Metrics { return Metrics{} }).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if !strings.HasPrefix(response.Body.String(), "kinosail_health 0\nkinosail_library_items 0\nkinosail_library_monitoring 0\n") {
		t.Fatalf("false metrics = %q", response.Body.String())
	}
}

func TestDiagnosticsHandlerWritesDownloadHeadersAndJSON(t *testing.T) {
	t.Parallel()
	response := httptest.NewRecorder()
	DiagnosticsHandler(func() DiagnosticReport { return DiagnosticReport{Version: "v9", Healthy: true} }).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	if response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Content-Disposition") != `attachment; filename="kinosail-diagnostics.json"` || response.Body.String() != `{"version":"v9","generated":"","uptimeSeconds":0,"lastScan":"","playbackMode":"","transcoder":"","scanFrequency":"","libraryMonitoring":"","healthy":true,"scanError":false,"libraryItems":0,"sessions":0,"transcodeCacheBytes":0,"httpRequests":0,"httpErrors":0,"httpPanics":0,"activityFailures":0,"activityHealthy":false}`+"\n" {
		t.Fatalf("diagnostics headers=%v body=%q", response.Header(), response.Body.String())
	}
}

func apiTaskRequest(t *testing.T, handler http.Handler, task string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/api/v1/tasks/"+task, nil)
	request.SetPathValue("task", task)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
