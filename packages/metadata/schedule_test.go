package metadata

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestScheduleCoalescesRefreshJobs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var observed func([]library.Item)
	var calls atomic.Int32
	release := make(chan struct{})
	Schedule(ctx, true, func(callback func([]library.Item)) { observed = callback }, func(context.Context) error {
		calls.Add(1)
		<-release
		return nil
	})
	if observed == nil {
		t.Fatal("scan observer was not registered")
	}
	observed(nil)
	waitForRefreshCalls(t, &calls, 1)
	observed(nil)
	observed(nil)
	close(release)
	waitForRefreshCalls(t, &calls, 2)
	time.Sleep(10 * time.Millisecond)
	if calls.Load() != 2 {
		t.Fatalf("refresh calls = %d, want 2", calls.Load())
	}
}

func TestScheduleRejectsUnavailableOrMissingAdapters(t *testing.T) {
	t.Parallel()
	called := false
	observe := func(func([]library.Item)) { called = true }
	refresh := func(context.Context) error { called = true; return nil }
	Schedule(t.Context(), false, observe, refresh)
	Schedule(nil, true, observe, refresh) //nolint:staticcheck // Explicitly proves nil is rejected.
	Schedule(t.Context(), true, nil, refresh)
	Schedule(t.Context(), true, observe, nil)
	if called {
		t.Fatal("invalid schedule registered or ran work")
	}
}

func waitForRefreshCalls(t *testing.T, calls *atomic.Int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if calls.Load() >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("refresh calls = %d, want %d", calls.Load(), want)
}
