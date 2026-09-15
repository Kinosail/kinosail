package auditjournal

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestActivityAndExportAPI(t *testing.T) { //nolint:cyclop // One compact contract test covers the two paired activity endpoints.
	t.Parallel()
	tracker, _ := trackerFixture(t)
	now := time.Now().UTC()
	tracker.Record(testEvent("security", "security", now))
	tracker.Record(testEvent("playback", "playback", now))
	progressCalls := 0
	var view ActivityView[string]
	status := 0
	handler := ActivityAPI(tracker, func() string {
		progressCalls++
		return "recent"
	}, func(_ http.ResponseWriter, value any, code int) {
		view = value.(ActivityView[string])
		status = code
	})
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity?category=security", nil))
	if status != http.StatusOK || progressCalls != 1 || view.Progress != "recent" || len(view.Events) != 1 || view.Events[0].ID != "security" || len(view.Audit) != 2 || len(view.Playback) != 1 {
		t.Fatalf("status=%d calls=%d view=%#v", status, progressCalls, view)
	}
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity?limit=1", nil))
	if len(view.Audit) != 1 {
		t.Fatalf("limited view = %#v", view)
	}
	response := httptest.NewRecorder()
	ExportAPI(tracker).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/activity/export", nil))
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/x-ndjson" {
		t.Fatalf("export = %d %v", response.Code, response.Header())
	}
}
