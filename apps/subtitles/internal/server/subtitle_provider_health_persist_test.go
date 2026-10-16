package server

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSubtitleProviderHealthPersistsChangesWithoutRewritingEverySuccess(t *testing.T) {
	t.Parallel()
	data := t.TempDir()
	path := filepath.Join(data, "subtitle_provider_health.json")
	now := time.Now().Add(-time.Hour).Truncate(time.Second)
	registry := newSubtitleProviderHealthRegistry(data)
	registry.now = func() time.Time { return now }
	success := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}
	registry.observe("SubDL", success, nil)
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	registry.observe("SubDL", success, nil)
	if repeated, err := os.ReadFile(path); err != nil || string(repeated) != string(first) {
		t.Fatalf("routine success rewrote health state: %q, %v", repeated, err)
	}
	registry.observe("SubDL", &http.Response{StatusCode: http.StatusServiceUnavailable, Header: http.Header{}}, nil)
	reloaded := newSubtitleProviderHealthRegistry(data)
	if state := reloaded.states["SubDL"]; state.failures != 1 || state.nextRetry.IsZero() {
		t.Fatalf("failure was not persisted: %#v", state)
	}
	now = now.Add(10 * time.Minute)
	registry.observe("SubDL", success, nil)
	reloaded = newSubtitleProviderHealthRegistry(data)
	if state := reloaded.states["SubDL"]; state.failures != 0 || !state.lastSuccess.Equal(now) {
		t.Fatalf("recovery was not persisted: %#v", state)
	}
	now = now.Add(6 * time.Minute)
	registry.observe("SubDL", success, nil)
	reloaded = newSubtitleProviderHealthRegistry(data)
	if state := reloaded.states["SubDL"]; !state.lastSuccess.Equal(now) {
		t.Fatalf("stale success time was never refreshed: %#v", state)
	}
}
