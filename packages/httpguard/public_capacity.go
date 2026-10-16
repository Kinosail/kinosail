package httpguard

import (
	"context"
	"sync"
	"time"
)

const (
	globalConcurrentLimit  = 128
	sourceConcurrentLimit  = 32
	prefixConcurrentLimit  = 64
	queuedRequestLimit     = 256
	sourceQueuedLimit      = 32
	publicCapacityPatience = 2 * time.Second
)

type PublicCapacity struct {
	mu       sync.Mutex
	active   int
	queued   int
	released chan struct{}
	sources  map[string]int
	prefixes map[string]int
	waiting  map[string]int
}

func NewPublicCapacity() *PublicCapacity {
	return &PublicCapacity{released: make(chan struct{}), sources: make(map[string]int), prefixes: make(map[string]int), waiting: make(map[string]int)}
}

func (capacity *PublicCapacity) Wait(ctx context.Context, key string, patience time.Duration) bool {
	capacity.mu.Lock()
	if capacity.acquireLocked(key) {
		capacity.mu.Unlock()
		return true
	}
	if capacity.queued >= queuedRequestLimit || capacity.waiting[key] >= sourceQueuedLimit {
		capacity.mu.Unlock()
		return false
	}
	capacity.queued++
	capacity.waiting[key]++
	released := capacity.released
	capacity.mu.Unlock()
	defer capacity.leaveQueue(key)
	timer := time.NewTimer(patience)
	defer timer.Stop()
	for {
		select {
		case <-released:
		case <-timer.C:
			return false
		case <-ctx.Done():
			return false
		}
		capacity.mu.Lock()
		acquired := capacity.acquireLocked(key)
		released = capacity.released
		capacity.mu.Unlock()
		if acquired {
			return true
		}
	}
}

func (capacity *PublicCapacity) acquireLocked(key string) bool {
	prefix := abusePrefix(key)
	if capacity.active >= globalConcurrentLimit || capacity.sources[key] >= sourceConcurrentLimit || prefix != "" && capacity.prefixes[prefix] >= prefixConcurrentLimit {
		return false
	}
	capacity.active++
	capacity.sources[key]++
	if prefix != "" {
		capacity.prefixes[prefix]++
	}
	return true
}

func (capacity *PublicCapacity) leaveQueue(key string) {
	capacity.mu.Lock()
	defer capacity.mu.Unlock()
	capacity.queued--
	capacity.waiting[key]--
	if capacity.waiting[key] == 0 {
		delete(capacity.waiting, key)
	}
}

func (capacity *PublicCapacity) Release(key string) {
	capacity.mu.Lock()
	defer capacity.mu.Unlock()
	capacity.active--
	capacity.sources[key]--
	if capacity.sources[key] == 0 {
		delete(capacity.sources, key)
	}
	if prefix := abusePrefix(key); prefix != "" {
		capacity.prefixes[prefix]--
		if capacity.prefixes[prefix] == 0 {
			delete(capacity.prefixes, prefix)
		}
	}
	if capacity.queued > 0 {
		close(capacity.released)
		capacity.released = make(chan struct{})
	}
}
