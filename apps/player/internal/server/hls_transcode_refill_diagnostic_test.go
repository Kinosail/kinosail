package server_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Preserve a bounded safe failure projection even when the public request fails
// before the ordinary clock receipts. Raw process and request logs stay private.
func transcodeRefillDiagnostics(t *testing.T, logs *copiedLogBuffer, tools string) {
	t.Helper()
	if !t.Failed() {
		return
	}
	var failures []map[string]any
	for _, line := range strings.Split(logs.snapshot(), "\n") {
		var record map[string]any
		if json.Unmarshal([]byte(line), &record) != nil || record["level"] != "WARN" && record["level"] != "ERROR" {
			continue
		}
		failures = append(failures, map[string]any{"level": record["level"], "operation": transcodeRefillFailureOperation(record)})
		if len(failures) == 16 {
			break
		}
	}
	transcodeRefillLog(t, map[string]any{"failureProjection": failures, "observedProducers": transcodeRefillProducers(t, tools)})
}

func transcodeRefillFailureOperation(record map[string]any) string {
	operations := []string{"source-clock", "rendition-clock", "completion-bounds-or-policy", "aac-packet-duration", "recovery-root", "recovery-master", "recovery-selection", "recovery-assets", "proof-digest", "proof-fields", "proof-encoder", "refill-window", "refill-encode-or-bound", "refill-stage-clock", "refill-generation", "refill-pending-clock", "refill-link", "refill-committed-clock"}
	detail, _ := record["error"].(string)
	for _, allowed := range operations {
		if record["failure_class"] == allowed || strings.Contains(detail, "("+allowed+")") {
			return allowed
		}
	}
	return "unclassified"
}
