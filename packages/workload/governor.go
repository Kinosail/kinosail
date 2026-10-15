package workload

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
)

type Class uint8

const (
	Playback Class = iota
	Background
)

type Governor struct {
	background                         chan struct{}
	mu                                 sync.Mutex
	capacity, used                     int
	changed                            chan struct{}
	devices                            map[string]bool
	activePlayback, activeBackground   atomic.Int64
	waitingPlayback, waitingBackground atomic.Int64
}

type Metrics struct {
	Capacity, BackgroundCapacity       int
	ActivePlayback, ActiveBackground   int64
	WaitingPlayback, WaitingBackground int64
}

func New(capacity int) *Governor {
	capacity = max(1, capacity)
	return &Governor{capacity: capacity, changed: make(chan struct{}), devices: make(map[string]bool), background: make(chan struct{}, max(1, capacity-2))}
}

func HeavyCapacity() int { return min(3, max(1, runtime.GOMAXPROCS(0)-1)) }

func (governor *Governor) Acquire(ctx context.Context, class Class) (func(), error) {
	return governor.AcquireEncoding(ctx, class, 1, "")
}

func (governor *Governor) AcquireEncoding(ctx context.Context, class Class, cost int, device string) (func(), error) {
	if !validEncodingReservation(class, cost, device) {
		return nil, errors.New("invalid encoding reservation")
	}
	if governor == nil {
		return func() {}, nil
	}
	if cost > governor.capacity {
		return nil, errors.New("encoding reservation exceeds capacity")
	}
	waiting, active := &governor.waitingPlayback, &governor.activePlayback
	if class == Background {
		waiting, active = &governor.waitingBackground, &governor.activeBackground
	}
	waiting.Add(1)
	defer waiting.Add(-1)
	if err := governor.encodingQueue(ctx, class); err != nil {
		return nil, err
	}
	if err := governor.waitEncoding(ctx, cost, device); err != nil {
		if class == Background {
			<-governor.background
		}
		return nil, err
	}
	active.Add(1)
	var once sync.Once
	return func() {
		once.Do(func() {
			active.Add(-1)
			governor.mu.Lock()
			governor.used -= cost
			delete(governor.devices, device)
			close(governor.changed)
			governor.changed = make(chan struct{})
			governor.mu.Unlock()
			if class == Background {
				<-governor.background
			}
		})
	}, nil
}

func (governor *Governor) Metrics() Metrics {
	return Metrics{governor.capacity, cap(governor.background), governor.activePlayback.Load(), governor.activeBackground.Load(), governor.waitingPlayback.Load(), governor.waitingBackground.Load()}
}

func (governor *Governor) EncodingCapacity() int {
	if governor == nil {
		return 1
	}
	return governor.capacity
}

func (governor *Governor) waitEncoding(ctx context.Context, cost int, device string) error {
	for {
		governor.mu.Lock()
		if ctx.Err() == nil && governor.used+cost <= governor.capacity && (device == "" || !governor.devices[device]) {
			governor.used += cost
			if device != "" {
				governor.devices[device] = true
			}
			governor.mu.Unlock()
			break
		}
		changed := governor.changed
		governor.mu.Unlock()
		select {
		case <-changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (governor *Governor) encodingQueue(ctx context.Context, class Class) error {
	if class == Background {
		select {
		case governor.background <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func validEncodingReservation(class Class, cost int, device string) bool {
	return cost >= 1 && len(device) <= 512 && (class == Playback || class == Background)
}
