package servertest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func AssertOperationalMetrics(t *testing.T, metrics *httptest.ResponseRecorder) {
	t.Helper()
	if metrics.Code != http.StatusOK || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_health")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_workload_capacity")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_workload_waiting{class=\"playback\"}")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_notification_dropped_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_viewing_sync_deferred_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_live_event_subscribers")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_watch_room_reconnects_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_watch_room_drift_seconds")) {
		t.Fatalf("metrics = %d %q", metrics.Code, metrics.Body.String())
	}
	assertRuntimeMetrics(t, metrics.Body.String())
}

func assertRuntimeMetrics(t *testing.T, body string) {
	t.Helper()
	values := map[string]float64{}
	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		name, raw, found := strings.Cut(line, " ")
		value, err := strconv.ParseFloat(raw, 64)
		if !found || err != nil {
			t.Fatalf("metric line %q is not a sample", line)
		}
		values[name] = value
	}
	for _, name := range []string{"kinosail_go_goroutines", "kinosail_go_memory_total_bytes", "kinosail_go_memory_limit_bytes", "kinosail_go_sched_latency_seconds_count"} {
		if values[name] <= 0 {
			t.Fatalf("%s = %v in %q", name, values[name], body)
		}
	}
	if values["kinosail_go_gomaxprocs"] != float64(runtime.GOMAXPROCS(0)) {
		t.Fatalf("kinosail_go_gomaxprocs = %v, want %d", values["kinosail_go_gomaxprocs"], runtime.GOMAXPROCS(0))
	}
	for _, name := range []string{"kinosail_go_heap_live_bytes", "kinosail_go_gc_cycles_total", "kinosail_go_gc_pause_seconds_count"} {
		if _, present := values[name]; !present {
			t.Fatalf("missing %s in %q", name, body)
		}
	}
	for _, summary := range []string{"kinosail_go_gc_pause_seconds", "kinosail_go_sched_latency_seconds"} {
		previous := -1.0
		for _, quantile := range []string{"0.5", "0.99", "1"} {
			value, present := values[summary+`{quantile="`+quantile+`"}`]
			if !present || value < previous {
				t.Fatalf("%s quantile %s = %v after %v in %q", summary, quantile, value, previous, body)
			}
			previous = value
		}
	}
}

func (fixture LibraryAPIFixture) OnboardingSettingsRequireExplicitBoolean(t *testing.T) {
	handler, token := fixture.Server(t)
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"onboardingPending":true`)
	for _, body := range []any{map[string]any{}, map[string]any{"enabled": true, "unexpected": true}} {
		response := APICall(t, handler, token, http.MethodPut, "/api/v1/settings/onboarding", body)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid onboarding settings = %d %q", response.Code, response.Body.String())
		}
	}
	AssertAPIBody(t, APICall(t, handler, token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"onboardingPending":true`)
}
