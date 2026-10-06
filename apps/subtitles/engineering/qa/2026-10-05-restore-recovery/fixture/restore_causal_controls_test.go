package main

// Controlled TLS seams, not populated-product acceptance. These test-first
// controls protect no-effects rejection, identical framing and live cancellation.
import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestRestoreCausalProbePublicControl(t *testing.T) {
	target := newRestoreTarget(t)
	f := newRestoreControl(t, target, "none")
	f.app = http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("diagnostic cannot call product handler") })
	before := f.snapshot()
	client := target.privateClient(3 * time.Second)
	assertRejectedCausalProbes(t, f, client, before)
	assertCausalWireEquality(t, f, client)
	assertLiveCausalCancellation(t, f, client)
	if f.snapshot() != before {
		t.Fatal("diagnostic changed Restore state")
	}
	raw, err := json.Marshal(f.causalSnapshot())
	if err != nil || len(raw) > 2048 {
		t.Fatal("closed probe output overflow")
	}
}

func assertRejectedCausalProbes(t *testing.T, f *restoreRig, client *http.Client, before restoreSnapshot) {
	t.Helper()
	for _, input := range []struct{ method, path, body string }{
		{http.MethodPost, "/__r06_restore/probe", `{"mode":"foreign"}`},
		{http.MethodPost, "/__r06_restore/probe", `{"mode":"direct","extra":true}`},
		{http.MethodGet, "/__r06_restore/probe", `{"mode":"direct"}`},
		{http.MethodPost, "/__r06_restore/probe?extra=1", `{"mode":"direct"}`},
		{http.MethodPost, "/__r06_restore/probe", strings.Repeat("x", 257)},
	} {
		request, err := http.NewRequestWithContext(t.Context(), input.method, f.target.origin+input.path, strings.NewReader(input.body))
		if err != nil {
			t.Fatal("fixed probe control unavailable")
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("probe rejection unavailable")
		}
		if _, err = readPrivateResponse(response); err != nil || response.StatusCode < 400 {
			t.Fatal("invalid probe admitted")
		}
		if f.snapshot() != before || f.causalSnapshot() != [3]restoreProbeSnapshot{} {
			t.Fatal("rejection caused effects")
		}
	}
}

func assertCausalWireEquality(t *testing.T, f *restoreRig, client *http.Client) {
	t.Helper()
	var previous http.Header
	for _, mode := range []string{"direct", "captured"} {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, f.target.origin+"/__r06_restore/probe", strings.NewReader(`{"mode":"`+mode+`"}`))
		if err != nil {
			t.Fatal("fixed probe unavailable")
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal("fixed probe request unavailable")
		}
		data, err := readPrivateResponse(response)
		if err != nil || response.StatusCode != http.StatusNoContent || len(data) != 0 {
			t.Fatal("probe changed empty204")
		}
		if previous != nil && !reflect.DeepEqual(previous, response.Header) {
			t.Fatal("direct/captured wire headers differ")
		}
		previous = response.Header.Clone()
	}
}

func assertLiveCausalCancellation(t *testing.T, f *restoreRig, client *http.Client) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, f.target.origin+"/__r06_restore/probe", strings.NewReader(`{"mode":"held"}`))
	if err != nil {
		t.Fatal("held probe unavailable")
	}
	request.Header.Set("Content-Type", "application/json")
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
	for !f.causalSnapshot()[2].Held && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !f.causalSnapshot()[2].Held {
		t.Fatal("held cancellation not armed")
	}
	cancel()
	select {
	case failed := <-completed:
		if !failed {
			t.Fatal("intentional abort did not reject")
		}
	case <-time.After(time.Second):
		t.Fatal("client cancellation unsettled")
	}
	for !f.causalSnapshot()[2].Settled && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	state := f.causalSnapshot()[2]
	if !state.Cancelled || !state.Settled || state.Delivered || state.TimedOut {
		t.Fatal("live handler did not witness cancellation")
	}
}

// Isolation gap: real network timing cannot force a select race deterministically.
func TestRestoreCausalCancellationBoundary(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	f := &restoreRig{ctx: ctx}
	if !f.causalCancellationQualifies(time.Now().Add(time.Second)) {
		t.Fatal("live cancellation qualifier rejected")
	}
	if f.causalCancellationQualifies(time.Now().Add(-time.Second)) {
		t.Fatal("expired cancellation qualified")
	}
	cancel()
	if f.causalCancellationQualifies(time.Now().Add(time.Second)) {
		t.Fatal("stopped lifecycle cancellation qualified")
	}
}
