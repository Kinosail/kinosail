package httpguard

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestPublicCapacityBoundsQueuedRequestsPerSource(t *testing.T) {
	t.Parallel()
	capacity := NewPublicCapacity()
	for range sourceConcurrentLimit {
		if !capacity.Acquire("192.0.2.9") {
			t.Fatal("source request was rejected before its limit")
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	var group sync.WaitGroup
	for range sourceQueuedLimit {
		group.Go(func() {
			if capacity.Wait(ctx, "192.0.2.9", time.Minute) {
				t.Error("queued request acquired a slot while the source was full")
			}
		})
	}
	deadline := time.Now().Add(time.Second)
	for queuedRequests(capacity) < sourceQueuedLimit {
		if time.Now().After(deadline) {
			t.Fatal("source requests were not queued")
		}
		time.Sleep(time.Millisecond)
	}
	started := time.Now()
	if capacity.Wait(t.Context(), "192.0.2.9", time.Minute) || time.Since(started) > time.Second {
		t.Fatal("source exceeded its queued request bound")
	}
	if !capacity.Wait(t.Context(), "192.0.2.10", time.Minute) {
		t.Fatal("one queued source blocked another source")
	}
	cancel()
	group.Wait()
	if queuedRequests(capacity) != 0 {
		t.Fatal("cancelled requests stayed queued")
	}
}

func queuedRequests(capacity *PublicCapacity) int {
	capacity.mu.Lock()
	defer capacity.mu.Unlock()
	return capacity.queued
}
