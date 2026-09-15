package auditjournal

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func trackerFixture(t *testing.T) (*HTTPTracker, *int) {
	t.Helper()
	marked := 0
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	journal := New(t.Context(), Config{DataDir: t.TempDir()})
	tracker := NewHTTPTracker(journal, HTTPConfig{
		Action: func(request *http.Request) (string, string) {
			if request.URL.Path == "/none" {
				return "", ""
			}
			if request.URL.Path == "/playback/one" {
				return "playback.changed", "playback"
			}
			return "session.created", "security"
		},
		Details: func(request *http.Request) map[string]string {
			if request.URL.Path == "/nil-details" {
				return nil
			}
			return map[string]string{"input": "safe"}
		},
		Actor: func(request *http.Request) Actor {
			return Actor{Name: request.Header.Get("X-Actor"), ID: request.Header.Get("X-Actor-ID")}
		},
		FallbackActor: func(*http.Request) Actor { return Actor{Name: "Fallback", ID: "fallback"} },
		Target:        func(request *http.Request) string { return request.URL.Path },
		MarkSecurity:  func(*http.Request) { marked++ },
		RequestID:     func(*http.Request) string { return "request" },
		Remote:        func(*http.Request) string { return "127.0.0.1" },
		NewID:         func() string { return "event" },
		Now:           func() time.Time { return now },
	})
	return tracker, &marked
}

func TestResponseWriterTracksStatusBytesAndUnwraps(t *testing.T) { //nolint:cyclop // One response-writer matrix covers optional interfaces and exact counters.
	t.Parallel()
	response := httptest.NewRecorder()
	writer := &ResponseWriter{ResponseWriter: response}
	writer.WriteHeader(http.StatusCreated)
	writer.WriteHeader(http.StatusInternalServerError)
	written, err := writer.Write([]byte("body"))
	second, secondErr := writer.Write([]byte("more"))
	if err != nil || secondErr != nil || written != 4 || second != 4 || writer.Status != http.StatusCreated || writer.Bytes != 8 || response.Code != http.StatusCreated || writer.Unwrap() != response {
		t.Fatalf("writer = %#v response=%#v written=%d err=%v", writer, response, written, err)
	}
	implicit := &ResponseWriter{ResponseWriter: httptest.NewRecorder()}
	if _, err := implicit.Write([]byte("ok")); err != nil || implicit.Status != http.StatusOK || implicit.Bytes != 2 {
		t.Fatalf("implicit writer = %#v, %v", implicit, err)
	}
}

func TestHTTPTrackerRecordsSuccessSnapshotsActorsAndTitles(t *testing.T) { //nolint:cyclop // Exact event assertions protect the shared audit contract.
	t.Parallel()
	tracker, _ := trackerFixture(t)
	if tracker.ResolveTitle("one") != "" {
		t.Fatal("unset title resolved")
	}
	tracker.SetTitle(func(id string) string { return "Title " + id })
	snapshots := 0
	tracker.SetSnapshot(func(*http.Request) map[string]string {
		snapshots++
		return map[string]string{"name": map[int]string{1: "B", 2: "C"}[snapshots]}
	})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/playback/one", nil)
	request.SetPathValue("id", "one")
	request.Header.Set("X-Actor", "Sam")
	request.Header.Set("X-Actor-ID", "viewer")
	request = request.WithContext(WithActor(request.Context(), "Automation"))
	response := httptest.NewRecorder()
	tracker.Track(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusCreated) }), response, request)
	events := tracker.Query("", 10)
	if len(events) != 1 || events[0].Actor != "Automation" || events[0].ActorID != "viewer" || events[0].Target != "Title one" || events[0].TargetID != "one" || events[0].Result != "success" || events[0].Details["before.name"] != "B" || events[0].Details["after.name"] != "C" || events[0].Details["status"] != "201" || events[0].Details["title"] != "Title one" {
		t.Fatalf("event = %#v", events)
	}
	tracker.SetTitle(nil)
	tracker.SetSnapshot(nil)
	if tracker.ResolveTitle("one") != "" {
		t.Fatal("cleared title resolved")
	}
}

func TestHTTPTrackerHandlesPassThroughFailuresAndFallbackActors(t *testing.T) { //nolint:cyclop // Exact event assertions protect failure behavior.
	t.Parallel()
	tracker, marked := trackerFixture(t)
	served := 0
	tracker.Track(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served++ }), httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/none", nil))
	if served != 1 || len(tracker.Query("", 10)) != 0 {
		t.Fatal("unclassified request was not passed through")
	}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/nil-details", strings.NewReader("name=Browser"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tracker.Track(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusUnauthorized) }), httptest.NewRecorder(), request)
	event := tracker.Query("", 1)[0]
	if event.Action != "access.denied" || event.Category != "security" || event.Result != "denied" || event.Actor != "Fallback" || event.ActorID != "fallback" || event.Details["attemptedAction"] != "session.created" || event.Details["status"] != "401" || *marked != 1 {
		t.Fatalf("denied event = %#v marked=%d", event, *marked)
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/session", strings.NewReader("name= Browser "))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	tracker.config.FallbackActor = func(*http.Request) Actor { return Actor{} }
	tracker.Track(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), httptest.NewRecorder(), request)
	if event := tracker.Query("", 1)[0]; event.Actor != "Browser" || event.Result != "success" || event.Details["status"] != "200" {
		t.Fatalf("session actor = %#v", event)
	}
	request = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/session", strings.NewReader("name= Browser "))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("X-Actor", "Header Actor")
	request.Header.Set("X-Actor-ID", "header")
	tracker.Track(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusBadRequest) }), httptest.NewRecorder(), request)
	if event := tracker.Query("", 1)[0]; event.Actor != "Header Actor" || event.Result != "failure" || event.Details["status"] != "400" {
		t.Fatalf("boundary failure event = %#v", event)
	}
}

func TestHTTPTrackerRecordsSecurityPasskeyAndPlaybackEvents(t *testing.T) { //nolint:cyclop // One sequence covers each specialized event type.
	t.Parallel()
	tracker, marked := trackerFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/security", nil)
	request.Header.Set("X-Actor", "Sam")
	request.Header.Set("X-Actor-ID", "viewer")
	tracker.Denied(request, "required")
	tracker.Tripwire(request, "scanner", true)
	tracker.Tripwire(request, "probe", false)
	tracker.PasskeyRisk(request, Actor{Name: "Owner", ID: "owner"}, []byte("credential"), 9, true, false)
	tracker.Playback(request, "playback.started", "movie", "Arrival", 42.25, false)
	tracker.Playback(request, "playback.completed", "movie", "Arrival", 90, true)
	events := tracker.Query("", 10)
	if len(events) != 6 || *marked != 3 || events[0].Action != "playback.completed" || events[0].Details["watched"] != "true" || events[1].Details["watched"] != "" || events[1].Details["seconds"] != "42.25" || events[2].Action != "passkey.risk" || events[2].Actor != "Owner" || events[2].Details["signCount"] != "9" || events[2].Details["backupEligible"] != "true" || events[3].Details["quarantined"] != "false" || events[4].Details["quarantined"] != "true" || events[5].Details["reason"] != "required" {
		t.Fatalf("events = %#v", events)
	}
	if tracker.TripwireWarning() != "Public scanner probe detected; local and WireGuard access remain available." || len(tracker.RecentLogs()) != 4 || !tracker.Healthy() || !tracker.Status().Healthy {
		t.Fatal("tracker views do not match the journal")
	}
}

func TestHTTPTrackerExportContextAndInvalidDependencies(t *testing.T) { //nolint:cyclop // One test verifies all nil-safe view methods.
	t.Parallel()
	tracker, _ := trackerFixture(t)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/security", nil)
	tracker.Denied(request, "required")
	response := httptest.NewRecorder()
	tracker.Export(response)
	if response.Header().Get("Content-Type") != "application/x-ndjson" || !strings.Contains(response.Header().Get("Content-Disposition"), "kinosail-activity.jsonl") || !strings.Contains(response.Body.String(), `"action":"access.denied"`) {
		t.Fatalf("export = headers=%v body=%q", response.Header(), response.Body.String())
	}
	var buffer bytes.Buffer
	if err := tracker.WriteJSONL(&buffer); err != nil || buffer.Len() == 0 {
		t.Fatalf("write = %q, %v", buffer.String(), err)
	}
	tracker.Export(nil)
	ctx := WithActor(context.Background(), "Robot")
	if ActorName(ctx) != "Robot" || ActorName(context.Background()) != "" {
		t.Fatal("actor context failed")
	}
	var invalid *HTTPTracker
	invalid.SetTitle(nil)
	invalid.SetSnapshot(nil)
	invalid.Track(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("invalid tracker served") }), httptest.NewRecorder(), request)
	invalid.Denied(request, "reason")
	invalid.Tripwire(request, "reason", false)
	invalid.PasskeyRisk(request, Actor{}, nil, 0, false, false)
	invalid.Playback(request, "", "", "", 0, false)
	invalid.Record(Event{})
	invalid.Export(nil)
	if invalid.ResolveTitle("") != "" || len(invalid.RecentLogs()) != 0 || invalid.TripwireWarning() != "" || invalid.Healthy() || len(invalid.Query("", 1)) != 0 || invalid.Status().Healthy || invalid.WriteJSONL(&buffer) != nil || invalid.Event(request, "", "", "", "", "", nil).ID != "" {
		t.Fatal("invalid tracker did not fail closed")
	}
	partial := NewHTTPTracker(nil, HTTPConfig{})
	partial.Track(nil, nil, nil)
	partial.Denied(nil, "")
	if partial.config.Now == nil || partial.validRequest(nil) || partial.valid() {
		t.Fatal("partial tracker became valid")
	}
	misconfigured := NewHTTPTracker(New(t.Context(), Config{}), HTTPConfig{})
	if misconfigured.valid() {
		t.Fatal("misconfigured tracker became valid")
	}
}
