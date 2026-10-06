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
}

func (b *copiedLogBuffer) Write(p []byte) (int, error) {
	b.Lock()
	defer b.Unlock()
	return b.Buffer.Write(p)
}
func (b *copiedLogBuffer) snapshot() string { b.Lock(); defer b.Unlock(); return b.String() }
func captureCopiedLogs(t *testing.T) *copiedLogBuffer {
	t.Helper()
	output := &copiedLogBuffer{}
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
