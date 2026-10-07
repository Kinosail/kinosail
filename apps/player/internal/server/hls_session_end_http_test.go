package server

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Failure modes: a closed page retains admission; malformed, unrelated-item,
// unrelated-session, or pause events cancel live work; the next movie stays queued.
// Controlled processes isolate admission and departure without claiming decode proof.
func phaseSessionDeparture(t *testing.T) {
	fixture := phaseFixture(t, 2, true)
	encoder := filepath.Join(filepath.Dir(fixture.starts), "ffmpeg")
	script, err := os.ReadFile(encoder)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(encoder, append(script, []byte("\nwhile :; do sleep .02; done\n")...), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fixture.release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if status := phaseStatus(t, fixture.request(t, 0, "t-a0-s0-none-t0-b0", "departure-first", "closing-session")); status != http.StatusOK {
		t.Fatalf("first movie startup = %d", status)
	}
	if status := phaseStatus(t, fixture.request(t, 0, "t-a0-s0-none-t0-b0", "departure-shared", "shared-session")); status != http.StatusOK {
		t.Fatalf("shared movie startup = %d", status)
	}
	next := fixture.request(t, 1, "a-a0-s0-none-t0-b0", "departure-next", "next-session")
	phaseWait(t, func() bool {
		return strings.Contains(strings.Join(phaseSequence(fixture.events("departure-next")), ","), "admission_wait")
	})
	for _, value := range []struct {
		name, item, payload string
		status              int
	}{
		{"missing session", fixture.ids[0], `{"event":"session-end","sequence":1}`, 400},
		{"malformed", fixture.ids[0], `{`, 400},
		{"unknown field", fixture.ids[0], `{"session":"closing-session","event":"session-end","sequence":1,"unknown":true}`, 400},
		{"oversized", fixture.ids[0], strings.Repeat(" ", 4096) + `{}`, 400},
		{"invalid sequence", fixture.ids[0], `{"session":"closing-session","event":"session-end","sequence":0}`, 400},
		{"unknown item", "missing-title", `{"session":"closing-session","event":"session-end","sequence":1}`, 404},
		{"other item", fixture.ids[1], `{"session":"closing-session","event":"session-end","sequence":1}`, 204},
		{"other session", fixture.ids[0], `{"session":"another-session","event":"session-end","sequence":1}`, 204},
		{"pause", fixture.ids[0], `{"session":"closing-session","event":"pause","sequence":2,"paused":true}`, 204},
	} {
		t.Run(value.name, func(t *testing.T) {
			if status := departureTrace(t, fixture, value.item, value.payload); status != value.status {
				t.Fatalf("trace status = %d, want %d", status, value.status)
			}
			select {
			case status := <-next:
				t.Fatalf("non-departure event released another encoder: %d", status)
			case <-time.After(30 * time.Millisecond):
			}
		})
	}
	if status := departureTrace(t, fixture, fixture.ids[0], `{"session":"closing-session","event":"session-end","sequence":3}`); status != http.StatusNoContent {
		t.Fatalf("departure trace = %d", status)
	}
	select {
	case status := <-next:
		t.Fatalf("departure interrupted a shared viewing session: %d", status)
	case <-time.After(50 * time.Millisecond):
	}
	// A request already dispatched by the departed page cannot reopen its attachment.
	if status := phaseStatus(t, fixture.request(t, 0, "t-a0-s0-none-t0-b0", "departure-late", "closing-session")); status != http.StatusOK {
		t.Fatalf("late cached request = %d", status)
	}
	if status := departureTrace(t, fixture, fixture.ids[0], `{"session":"shared-session","event":"session-end","sequence":1}`); status != http.StatusNoContent {
		t.Fatalf("last shared departure trace = %d", status)
	}
	status := phaseStatus(t, next)
	if status != http.StatusOK {
		t.Fatalf("next movie could not start after departure: %d", status)
	}
	phaseReceipt(t, status, fixture.events("departure-next"))
}

func departureTrace(t *testing.T, fixture phaseHTTPFixture, item, payload string) int {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, fmt.Sprintf("%s/api/v1/items/%s/playback-events", fixture.host.URL, item), bytes.NewBufferString(payload))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := fixture.client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	return response.StatusCode
}
