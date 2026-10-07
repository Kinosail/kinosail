package server_test

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestHLSSpeedPhaseWitnessSurvivesUnrelatedRequestChatter(t *testing.T) {
	output := captureCopiedLogs(t, 64<<10)
	slog.Info("HLS encode phase", "phase", "admitted", "request_id", "fixture-request", "mode", "remux", "elapsed_ms", 12, "error", "private-error")
	for index := range 80 {
		slog.Info("request", "private", fmt.Sprintf("secret-%d", index))
	}
	facts := speedFailureOperationFacts(output.snapshot(), "fixture-request")
	if len(facts) != 1 || facts[0]["phase"] != "admitted" || facts[0]["request_matches"] != true || facts[0]["session_present"] != false {
		t.Fatalf("bounded phase was lost behind unrelated requests: %+v", facts)
	}
	data, _ := json.Marshal(facts)
	if strings.Contains(string(data), "private") || strings.Contains(string(data), "fixture-request") || strings.Contains(string(data), "secret") {
		t.Fatal("phase projection leaked private fields")
	}
}

func TestHLSSpeedPhaseWitnessBoundsRelevantEventsAndEviction(t *testing.T) {
	output := &copiedLogBuffer{limit: 64 << 10}
	for index := range 60 {
		_, err := fmt.Fprintf(output, "{\"msg\":\"HLS transcode started\",\"phase\":\"process_started\",\"elapsed_ms\":%d}\n", index)
		if err != nil {
			t.Fatal(err)
		}
	}
	facts := speedFailureOperationFacts(output.snapshot(), "fixture-request")
	if len(facts) != 40 || facts[0]["elapsed_ms"] != float64(20) || facts[39]["elapsed_ms"] != float64(59) {
		t.Fatal("relevant event projection is not capped at the latest forty")
	}
	if _, err := output.Write([]byte(strings.Repeat("x", 64<<10))); err != nil {
		t.Fatal(err)
	}
	if len(speedFailureOperationFacts(output.snapshot(), "fixture-request")) != 0 {
		t.Fatal("evicted events were fabricated")
	}
	for _, entry := range []map[string]any{
		{"msg": "HLS encode phase", "phase": "private-phase", "outcome": "private-error", "elapsed_ms": -1.0},
		{"msg": "HLS encode phase", "phase": "private-phase", "elapsed_ms": 3600001.0},
	} {
		data, _ := json.Marshal(speedFailureOperationFields(entry, "fixture-request"))
		if strings.Contains(string(data), "private") || strings.Contains(string(data), "elapsed_ms") {
			t.Fatal("unbounded or unknown phase fields admitted")
		}
	}
}
