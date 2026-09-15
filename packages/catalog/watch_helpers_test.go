package catalog

import (
	"context"
	"testing"
	"time"
)

func runPoll(t *testing.T, index *Index) (context.CancelFunc, <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		index.poll(ctx)
		close(done)
	}()
	return cancel, done
}

func waitCall(t *testing.T, calls <-chan int, wanted int) {
	t.Helper()
	for {
		select {
		case call := <-calls:
			if call >= wanted {
				return
			}
		case <-time.After(time.Second):
			t.Fatalf("snapshot call %d did not run", wanted)
		}
	}
}
