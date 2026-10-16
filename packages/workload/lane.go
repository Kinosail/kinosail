package workload

import (
	"context"
	"slices"
	"sync"
)

const classCount = 3

type lane struct {
	mu       sync.Mutex
	capacity int
	strict   bool
	limits   [classCount]int
	used     [classCount]int
	waiters  [classCount][]*laneWaiter
}

type laneWaiter struct {
	cost  int
	ready chan struct{}
}

func newLane(capacity int, strict bool, interactive, background int) *lane {
	return &lane{capacity: capacity, strict: strict, limits: [classCount]int{capacity, interactive, background}}
}

func rank(class Class) int {
	switch class {
	case Playback:
		return 0
	case Interactive:
		return 1
	case Background:
	}
	return 2
}

func (lane *lane) possible(priority, cost int) bool {
	return cost <= lane.capacity && (!lane.strict || cost <= lane.limits[priority])
}

func (lane *lane) fits(priority, cost int) bool {
	if lane.used[priority]+cost > lane.limits[priority] && (lane.strict || lane.used[priority] > 0) {
		return false
	}
	total := cost
	for other, used := range lane.used {
		if lane.strict || other <= priority {
			total += used
		}
	}
	return total <= lane.capacity
}

func (lane *lane) acquire(ctx context.Context, priority, cost int) error {
	lane.mu.Lock()
	if len(lane.waiters[priority]) == 0 && lane.fits(priority, cost) {
		lane.used[priority] += cost
		lane.mu.Unlock()
		return nil
	}
	waiter := &laneWaiter{cost: cost, ready: make(chan struct{})}
	lane.waiters[priority] = append(lane.waiters[priority], waiter)
	lane.mu.Unlock()
	select {
	case <-waiter.ready:
		return nil
	case <-ctx.Done():
	}
	lane.mu.Lock()
	defer lane.mu.Unlock()
	select {
	case <-waiter.ready:
		lane.used[priority] -= cost
	default:
		lane.waiters[priority] = slices.DeleteFunc(lane.waiters[priority], func(candidate *laneWaiter) bool { return candidate == waiter })
	}
	lane.grant()
	return ctx.Err()
}

func (lane *lane) release(priority, cost int) {
	lane.mu.Lock()
	lane.used[priority] -= cost
	lane.grant()
	lane.mu.Unlock()
}

func (lane *lane) grant() {
	for priority := range lane.waiters {
		for len(lane.waiters[priority]) > 0 && lane.fits(priority, lane.waiters[priority][0].cost) {
			waiter := lane.waiters[priority][0]
			lane.waiters[priority] = lane.waiters[priority][1:]
			lane.used[priority] += waiter.cost
			close(waiter.ready)
		}
	}
}
