package operations

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestFailureLogBoundsAndRedactsUntrustedRequestValues(t *testing.T) { //nolint:cyclop // One privacy matrix covers malformed fields and benign outcomes.
	t.Parallel()
	var log FailureLog
	const requestID = "550e8400-e29b-41d4-a716-446655440000"
	log.Record(requestID, "GET", "/api/v1/items/private-title/playback?token=private-secret", 503, 2*time.Second)
	log.Record("bad\nsecret", "GET private-secret", "/private-secret", 404, time.Second)
	log.Record("token-looking-value-123456789012", "GET", "/api/v1/library", 500, time.Second)
	log.Record(requestID, "GET", "/api/v1/library", 200, time.Second)
	log.Record(requestID, "GET", "/api/v1/library", 700, time.Second)
	events := log.Recent()
	if len(events) != 3 || events[2].Operation != "items-playback" || events[2].Level != "error" || events[2].Status != 503 || events[2].RequestID != requestID || events[1].Level != "warn" || events[0].RequestID != "" {
		t.Fatalf("failure events = %#v", events)
	}
	encoded, err := json.Marshal(events)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"private-title", "private-secret", "bad\\nsecret", "token", "/api/v1/items/"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("failure log exposed %q: %s", private, encoded)
		}
	}
}

func TestFailureLogKeepsOnlyFiftyNewestEvents(t *testing.T) {
	t.Parallel()
	var log FailureLog
	for index := range 60 {
		log.Record(fmt.Sprintf("%024x", index), "GET", "/api/v1/library", 500, time.Second)
	}
	events := log.Recent()
	if len(events) != 50 || events[0].RequestID != fmt.Sprintf("%024x", 59) || events[49].RequestID != fmt.Sprintf("%024x", 10) {
		t.Fatalf("recent failures = %#v", events)
	}
}
