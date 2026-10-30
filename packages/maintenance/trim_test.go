package maintenance

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunTrimsBoundedCachesOnlyWhenIdleAndReportsFailures(t *testing.T) {
	t.Parallel()
	manager := New(nil, time.Hour, 1, (&fixture{}).dependencies()) //nolint:staticcheck // A nil context is the documented schedule-off mode.
	manager.now = func() time.Time { return time.Unix(0, 100) }
	if err := manager.Run(); err != nil {
		t.Fatalf("run without a trim = %v", err)
	}
	var trims atomic.Int64
	failure := errors.New("trim failed")
	manager.SetTrim(func() error {
		trims.Add(1)
		return failure
	})

	manager.foreground.Add(1)
	if err := manager.Run(); err != nil || trims.Load() != 0 {
		t.Fatalf("busy run trimmed caches: trims=%d err=%v", trims.Load(), err)
	}
	manager.foreground.Add(-1)

	if err := manager.Run(); !errors.Is(err, failure) || trims.Load() != 1 {
		t.Fatalf("idle run: trims=%d err=%v", trims.Load(), err)
	}
}
