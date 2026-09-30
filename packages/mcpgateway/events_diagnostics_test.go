package mcpgateway

import (
	"bytes"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type eventLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (buffer *eventLogBuffer) Write(data []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.Write(data)
}

func (buffer *eventLogBuffer) text() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func TestEventsFailureDiagnosticsAreCorrelatedAndSafe(t *testing.T) {
	var logs eventLogBuffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	fixture := newEventFixture(t)
	result := fixture.subscribe(t, "library.updated", map[string]any{})
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
	if logs.text() != "" {
		t.Fatalf("successful delivery logged a failure: %s", logs.text())
	}
	fixture.status.Store(http.StatusGone)
	fixture.gateway.PublishEvent(t.Context(), "", "library.updated", "/api/v1/library")
	expectEvent(t, fixture)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && !strings.Contains(logs.text(), "http_4xx") {
		time.Sleep(time.Millisecond)
	}
	fields := map[string]any{"name": "library.updated", "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.test/bad-challenge", "secret": fixture.secret}}
	mcpRequest(t, fixture.mux, "events/subscribe", fields)
	output := logs.text()
	for _, expected := range []string{`"level":"WARN"`, `"operation":"events/deliver"`, `"operation":"events/subscribe"`, `"failure":"http_4xx"`, `"failure":"challenge_failed"`, result["id"].(string)} {
		if !strings.Contains(output, expected) {
			t.Errorf("missing diagnostic %s: %s", expected, output)
		}
	}
	for _, secret := range []string{fixture.secret, "receiver.example.test", "bad-challenge", `"token"`, `"data"`, "wrong"} {
		if strings.Contains(output, secret) {
			t.Errorf("unsafe diagnostic %s: %s", secret, output)
		}
	}
}
