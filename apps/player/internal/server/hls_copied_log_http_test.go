package server_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"
)

// Existing structured operational completion is the barrier: cancelling before
// certification finished could hide a late encode after malformed probe output.
type copiedLogBuffer struct {
	sync.Mutex
	bytes.Buffer
	limit int
}

func (b *copiedLogBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	length := len(p)
	if b.limit > 0 {
		if len(p) > b.limit {
			p = p[len(p)-b.limit:]
		}
		if extra := b.Len() + len(p) - b.limit; extra > 0 {
			b.Next(extra)
		}
	}
	_, err := b.Buffer.Write(p)
	return length, err
}
func (b *copiedLogBuffer) snapshot() string { b.Lock(); defer b.Unlock(); return b.String() }
func captureCopiedLogs(t *testing.T, limit ...int) *copiedLogBuffer {
	t.Helper()
	output := &copiedLogBuffer{}
	if len(limit) > 1 || len(limit) == 1 && (limit[0] <= 0 || limit[0] > 64<<10) {
		t.Fatal("invalid test log capture bound")
	}
	if len(limit) == 1 {
		output.limit = limit[0]
	}
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return output
}

func awaitCopiedLog(t *testing.T, output *copiedLogBuffer, message, requestID, state string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		for _, line := range bytes.Split([]byte(output.snapshot()), []byte("\n")) {
			var entry map[string]any
			if json.Unmarshal(line, &entry) == nil && entry["msg"] == message && (requestID == "" || entry["request_id"] == requestID) && (state == "" || entry["state"] == state) {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("operational completion missing: %s state=%s", message, state)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
