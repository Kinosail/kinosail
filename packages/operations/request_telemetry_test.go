package operations

import (
	"net/http"
	"sync"
	"testing"
	"time"
)

func TestRequestTelemetryCountsRequestsErrorsAndPanics(t *testing.T) {
	t.Parallel()
	var telemetry RequestTelemetry
	telemetry.Observe("one", http.MethodGet, "/api/v1/library", http.StatusOK, time.Millisecond, false)
	telemetry.Observe("two", http.MethodPost, "/api/v1/settings", http.StatusForbidden, time.Millisecond, false)
	telemetry.Observe("three", http.MethodGet, "/api/v1/library", http.StatusInternalServerError, time.Millisecond, true)
	requests, failed, panics := telemetry.Counts()
	if requests != 3 || failed != 2 || panics != 1 {
		t.Fatalf("counts = %d requests, %d errors, %d panics", requests, failed, panics)
	}
	recent := telemetry.Recent()
	if len(recent) != 2 || recent[0].Status != http.StatusInternalServerError || recent[0].Level != "error" || recent[1].Status != http.StatusForbidden || recent[1].Method != http.MethodPost {
		t.Fatalf("recent failures = %#v", recent)
	}
}

func TestRequestTelemetryIsSafeForConcurrentRequests(t *testing.T) {
	t.Parallel()
	var telemetry RequestTelemetry
	var group sync.WaitGroup
	for range 50 {
		group.Go(func() { telemetry.Observe("id", http.MethodGet, "/", http.StatusNotFound, 0, false) })
	}
	group.Wait()
	if requests, failed, _ := telemetry.Counts(); requests != 50 || failed != 50 {
		t.Fatalf("counts = %d, %d", requests, failed)
	}
}
