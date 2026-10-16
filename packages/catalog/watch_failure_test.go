package catalog

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestPersistentWatcherFailureBacksOffWithoutRepeatedFullScans(t *testing.T) {
	index := NewMemoryIndex(nil, true)
	index.pollInterval, index.watchRetry = time.Hour, 10*time.Millisecond
	streamClosed, watchLimit := errors.New("filesystem event stream closed"), errors.New("no space left on device")
	afterHealthyFailure := make(chan bool, 1)
	var calls atomic.Int32
	index.watch = func(context.Context) error {
		switch calls.Add(1) {
		case 1:
			index.setMonitoring(true, nil)
			return streamClosed
		case 2:
			afterHealthyFailure <- index.takeRequest().full
		}
		return watchLimit
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	index.Watch(ctx)
	select {
	case full := <-afterHealthyFailure:
		if !full {
			t.Fatal("losing a healthy watcher did not request one full scan")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("watcher was not retried")
	}
	time.Sleep(300 * time.Millisecond)
	cancel()
	if index.takeRequest().full {
		t.Fatal("a watcher that never recovered kept requesting full scans")
	}
	if attempts := calls.Load(); attempts > 7 {
		t.Fatalf("watcher retried %d times in about 300ms without backing off", attempts)
	}
	if watching, err := index.Monitoring(); watching || !errors.Is(err, watchLimit) {
		t.Fatalf("monitoring = %t, %v", watching, err)
	}
}
