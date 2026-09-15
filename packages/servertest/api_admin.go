package servertest

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

// AssertOperationalMetrics verifies the complete operational metric surface.
func AssertOperationalMetrics(t *testing.T, metrics *httptest.ResponseRecorder) {
	t.Helper()
	if metrics.Code != http.StatusOK || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_health")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_workload_capacity")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_workload_waiting{class=\"playback\"}")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_notification_dropped_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_viewing_sync_deferred_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_live_event_subscribers")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_watch_room_reconnects_total")) || !bytes.Contains(metrics.Body.Bytes(), []byte("kinosail_watch_room_drift_seconds")) {
		t.Fatalf("metrics = %d %q", metrics.Code, metrics.Body.String())
	}
}

// OnboardingSettingsRequireExplicitBoolean verifies strict decoding and no side effects.
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
