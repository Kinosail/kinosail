package playback

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestWaitHLSReadyPreservesPlayerPolling(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	calls := 0
	if !WaitHLSReady(ctx, func() error {
		calls++
		if calls == 1 {
			return os.ErrNotExist
		}
		return nil
	}) || calls != 2 {
		t.Fatalf("delayed readiness calls = %d", calls)
	}
	calls = 0
	if WaitHLSReady(ctx, func() error { calls++; return os.ErrPermission }) || calls != 1 {
		t.Fatalf("permanent error calls = %d", calls)
	}
	canceled, stop := context.WithCancel(t.Context())
	stop()
	if WaitHLSReady(canceled, func() error { return os.ErrNotExist }) {
		t.Fatal("canceled missing file became ready")
	}
	if !WaitHLSReady(canceled, func() error { return nil }) {
		t.Fatal("already-ready file changed canceled-context behavior")
	}
}
