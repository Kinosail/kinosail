package main

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// This real TLS control forces lifecycle termination while the client remains
// live. Request cancellation alone cannot deterministically expose an implicit
// successful response racing the client's transport cancellation.
func TestRestoreStoppedCausalProbeRejectsPublicControl(t *testing.T) {
	target := newRestoreTarget(t)
	f := newRestoreControl(t, target, "none")
	before := f.snapshot()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
		target.origin+"/__r06_restore/probe", strings.NewReader(`{"mode":"held"}`))
	if err != nil {
		t.Fatal("fixed held probe unavailable")
	}
	request.Header.Set("Content-Type", "application/json")
	client := target.privateClient(3 * time.Second)
	completed := make(chan bool, 1)
	go func() {
		response, err := client.Do(request)
		if response != nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
		}
		completed <- err != nil
	}()
	deadline := time.Now().Add(2 * time.Second)
	waitCausalProbeControl(t, f, deadline, false)
	f.cancel()
	select {
	case rejected := <-completed:
		if !rejected {
			t.Fatal("stopped held diagnostic returned an HTTP response")
		}
	case <-time.After(time.Second):
		t.Fatal("stopped held diagnostic did not settle")
	}
	waitCausalProbeControl(t, f, deadline, true)
	state := f.causalSnapshot()[2]
	for name, valid := range map[string]bool{
		"settled": state.Settled, "not delivered": !state.Delivered,
		"not client-cancelled": !state.Cancelled, "lifecycle ended": state.TimedOut,
	} {
		if !valid {
			t.Errorf("stopped diagnostic lost its non-delivery witness: %s", name)
		}
	}
	if f.snapshot() != before {
		t.Fatal("diagnostic termination changed Restore state")
	}
}
