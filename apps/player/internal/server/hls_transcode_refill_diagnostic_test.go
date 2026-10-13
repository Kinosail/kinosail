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
		failures = append(failures, map[string]any{"level": record["level"], "operation": transcodeRefillFailureOperation(record), "clock": transcodeRefillClockFields(record)})
		if len(failures) == 16 {
			break
		}
	}
	transcodeRefillLog(t, map[string]any{"failureProjection": failures, "observedProducers": transcodeRefillProducers(t, tools)})
}

func transcodeRefillClockFields(record map[string]any) map[string]any {
	clock, _ := record["clock"].(map[string]any)
	safe := make(map[string]any)
	for _, key := range []string{"measured", "init_equal", "video_clock_equal", "audio_clock_equal"} {
		if value, ok := clock[key].(bool); ok {
			safe[key] = value
		}
	}
	for _, key := range []string{"expected_rate", "actual_rate", "expected_base", "actual_base", "expected_video_pts", "actual_video_pts", "expected_video_dts", "actual_video_dts", "expected_audio_pts", "actual_audio_pts", "expected_video_packets", "actual_video_packets", "expected_audio_packets", "actual_audio_packets", "actual_cuts", "packet_stream", "packet_pts", "packet_dts", "packet_duration", "packet_position", "packet_size", "packet_owner", "first_audio_pts", "audio_packets", "video_packets"} {
		if value, ok := clock[key].(float64); ok && value >= -(1<<40) && value <= 1<<40 {
			safe[key] = value
		}
	}
	return safe
}

func transcodeRefillFailureOperation(record map[string]any) string {
	operations := []string{"source-clock", "rendition-clock", "completion-bounds-or-policy", "aac-packet-duration", "recovery-root", "recovery-master", "recovery-selection", "recovery-assets", "proof-digest", "proof-fields", "proof-encoder", "refill-window", "refill-encode-or-bound", "refill-stage-clock", "refill-generation", "refill-pending-clock", "refill-link", "refill-committed-clock", "probe-stream", "probe-record", "probe-owner", "probe-packets", "probe-values", "probe-keyframe", "probe-aac-sequence", "probe-packet-count", "probe-interval", "probe-run", "probe-consumption", "probe-mutated", "probe-streams", "probe-empty-track", "probe-init", "probe-field-stream_index", "probe-field-pts", "probe-field-dts", "probe-field-duration", "probe-field-pos", "probe-field-size"}
	detail, _ := record["error"].(string)
	for _, allowed := range operations {
		if record["failure_class"] == allowed || strings.Contains(detail, "("+allowed+")") {
			return allowed
		}
	}
	return "unclassified"
}
