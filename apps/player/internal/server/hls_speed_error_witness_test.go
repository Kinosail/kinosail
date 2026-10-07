package server_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Serialized diagnostics have no errors.Is identity. Fixed categories preserve
// the observable reason without retaining arbitrary paths or error strings.
func TestHLSSpeedErrorWitnessRetainsSafeFailureCategory(t *testing.T) {
	for reason, category := range map[string]string{
		"context canceled":                                         "context_canceled",
		"context deadline exceeded":                                "context_deadline",
		"HLS segment is invalid":                                   "segment_invalid",
		"playback recipe does not match the source":                "source_mismatch",
		"this Dolby Vision conversion is unsupported":              "source_unsupported",
		"playback settings changed; start a new compatible stream": "settings_mismatch",
		"HLS source or playback policy changed":                    "source_changed",
		"HLS source snapshot changed":                              "source_changed",
		"HLS source policy is invalid":                             "source_invalid",
		"HLS source is not a regular media file":                   "source_invalid",
		"copied HLS timeline is unavailable":                       "copied_index",
		"HLS segment is outside the playable duration":             "playable_duration",
		"video codec is not available on this Server":              "encoder_unavailable",
		"video encoder is not available on this Server":            "encoder_unavailable",
		"open <private>/index.m3u8: no such file or directory":     "filesystem_missing",
		"open <private>/index.m3u8: permission denied":             "filesystem_permission",
		"read <private>/index.m3u8: input/output error":            "filesystem_io",
		"unknown private fixture detail":                           "other",
	} {
		t.Run(category+"/"+reason[:min(12, len(reason))], func(t *testing.T) {
			entry := map[string]any{
				"msg": "HLS segment preparation failed", "error": reason,
				"cached": true, "request_context_done": false, "request_id": "fixture-request", "playback_session": "hls-speed-fixture",
			}
			facts := speedErrorWitnessFacts(t, entry)
			if facts["error_category"] != category || facts["cached"] != true || facts["request_context_done"] != false {
				t.Fatalf("safe failure facts absent: %+v", facts)
			}
			if facts["request_matches"] != true || facts["session_matches"] != true {
				t.Fatal("correlation booleans were lost")
			}
		})
	}
}

func TestHLSSpeedErrorWitnessRejectsMalformedFieldsAndKeepsContextIndependent(t *testing.T) {
	for _, entry := range []map[string]any{
		{"error": false, "cached": "true", "request_context_done": "false"},
		{"error": 42, "cached": 1, "request_context_done": nil},
		{"error": ""},
		{"error": strings.Repeat("private", 1200)},
		{},
	} {
		entry["msg"] = "HLS segment preparation failed"
		facts := speedErrorWitnessFacts(t, entry)
		for _, field := range []string{"error_category", "cached", "request_context_done"} {
			if _, exists := facts[field]; exists {
				t.Fatalf("malformed %s admitted", field)
			}
		}
	}
	facts := speedErrorWitnessFacts(t, map[string]any{"msg": "HLS segment preparation failed", "error": "HLS segment is outside the playable duration", "cached": false, "request_context_done": true})
	if facts["error_category"] != "playable_duration" || facts["cached"] != false || facts["request_context_done"] != true {
		t.Fatal("context observation replaced the serialized error reason")
	}
	other := speedErrorWitnessFacts(t, map[string]any{"msg": "HLS encode phase", "error": "context canceled", "cached": true, "request_context_done": true})
	if _, exists := other["error_category"]; exists {
		t.Fatal("unrelated phase classified as segment admission")
	}
}

func speedErrorWitnessFacts(t *testing.T, entry map[string]any) map[string]any {
	t.Helper()
	before, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	facts := speedFailureOperationFacts(string(before)+"\n", "fixture-request")
	if len(facts) != 1 {
		t.Fatalf("expected one bounded event, got %d", len(facts))
	}
	after, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("input log mutated")
	}
	output, err := json.Marshal(facts)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(output), "private") || strings.Contains(string(output), "fixture-request") || strings.Contains(string(output), "hls-speed-fixture") {
		t.Fatal("failure projection retained a private value")
	}
	if _, exists := facts[0]["error"]; exists {
		t.Fatal("raw error retained")
	}
	return facts[0]
}

func speedFailureErrorFields(entry, facts map[string]any) {
	if entry["msg"] != "HLS segment preparation failed" {
		return
	}
	for _, field := range []string{"cached", "request_context_done"} {
		if value, ok := entry[field].(bool); ok {
			facts[field] = value
		}
	}
	if reason, ok := entry["error"].(string); ok && len(reason) > 0 && len(reason) <= 8<<10 {
		facts["error_category"] = speedFailureErrorCategory(reason)
	}
}

var speedFailureErrorCategories = map[string]string{
	"context canceled":                                         "context_canceled",
	"context deadline exceeded":                                "context_deadline",
	"HLS segment is invalid":                                   "segment_invalid",
	"playback recipe does not match the source":                "source_mismatch",
	"this Dolby Vision conversion is unsupported":              "source_unsupported",
	"playback settings changed; start a new compatible stream": "settings_mismatch",
	"HLS source or playback policy changed":                    "source_changed",
	"HLS source snapshot changed":                              "source_changed",
	"HLS source policy is invalid":                             "source_invalid",
	"HLS source is not a regular media file":                   "source_invalid",
	"copied HLS timeline is unavailable":                       "copied_index",
	"HLS segment is outside the playable duration":             "playable_duration",
	"video codec is not available on this Server":              "encoder_unavailable",
	"video encoder is not available on this Server":            "encoder_unavailable",
}

func speedFailureErrorCategory(reason string) string {
	if category, known := speedFailureErrorCategories[reason]; known {
		return category
	}
	// These are coarse serialized OS reasons, never typed identity or path proof.
	for suffix, category := range map[string]string{
		": no such file or directory": "filesystem_missing",
		": permission denied":         "filesystem_permission",
		": input/output error":        "filesystem_io",
	} {
		if strings.HasSuffix(reason, suffix) {
			return category
		}
	}
	return "other"
}
