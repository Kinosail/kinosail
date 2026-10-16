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
	Interactive
)

type Governor struct {
	cpu, light      *lane
	mu              sync.Mutex
	devices         map[string]*lane
	sessions        func(string) int
	active, waiting [classCount]atomic.Int64
}

type Metrics struct {
	Capacity, BackgroundCapacity, InteractiveCapacity, LightCapacity int
	ActivePlayback, ActiveBackground, ActiveInteractive              int64
	WaitingPlayback, WaitingBackground, WaitingInteractive           int64
}

type Option func(*Governor)

func DeviceSessions(sessions func(string) int) Option {
	return func(governor *Governor) { governor.sessions = sessions }
}

func New(capacity int, options ...Option) *Governor {
	capacity = max(1, capacity)
	light := min(64, max(8, 2*capacity))
	governor := &Governor{
		cpu:      newLane(capacity, false, max(1, capacity/4), max(1, capacity/2)),
		light:    newLane(light, false, light/2, light/2),
		devices:  make(map[string]*lane),
		sessions: func(string) int { return 2 },
	}
	for _, option := range options {
		option(governor)
	}
	return governor
}

func HeavyCapacity() int { return capacityFor(runtime.GOMAXPROCS(0)) }

func capacityFor(processors int) int { return max(1, processors) }

func (governor *Governor) Acquire(ctx context.Context, class Class) (func(), error) {
	return governor.AcquireEncoding(ctx, class, 1, "")
}

func (governor *Governor) AcquireLight(ctx context.Context, class Class) (func(), error) {
	if !validClass(class) {
		return nil, errors.New("invalid workload class")
	}
	if governor == nil {
		return func() {}, nil
	}
	return governor.reserve(ctx, class, []laneCost{{governor.light, 1}})
}

func (governor *Governor) AcquireEncoding(ctx context.Context, class Class, cost int, device string) (func(), error) {
	if cost < 1 || len(device) > 512 || !validClass(class) {
		return nil, errors.New("invalid encoding reservation")
	}
	if governor == nil {
		return func() {}, nil
	}
	if device == "" {
		return governor.reserve(ctx, class, []laneCost{{governor.cpu, cost}})
	}
	return governor.reserve(ctx, class, []laneCost{{governor.device(device), cost}, {governor.light, 1}})
}

type laneCost struct {
	lane *lane
	cost int
}

func (governor *Governor) reserve(ctx context.Context, class Class, costs []laneCost) (func(), error) {
	priority := rank(class)
	for _, reservation := range costs {
		if !reservation.lane.possible(priority, reservation.cost) {
			return nil, errors.New("encoding reservation exceeds capacity")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	governor.waiting[priority].Add(1)
	defer governor.waiting[priority].Add(-1)
	for index, reservation := range costs {
		if err := reservation.lane.acquire(ctx, priority, reservation.cost); err != nil {
			releaseLanes(costs[:index], priority)
			return nil, err
		}
	}
	governor.active[priority].Add(1)
	var once sync.Once
	return func() {
		once.Do(func() {
			governor.active[priority].Add(-1)
			releaseLanes(costs, priority)
		})
	}, nil
}

func releaseLanes(costs []laneCost, priority int) {
	for index := len(costs) - 1; index >= 0; index-- {
		costs[index].lane.release(priority, costs[index].cost)
	}
}

func (governor *Governor) device(key string) *lane {
	governor.mu.Lock()
	defer governor.mu.Unlock()
	device := governor.devices[key]
	if device == nil {
		sessions := max(1, governor.sessions(key))
		device = newLane(sessions, true, sessions-1, sessions-1)
		governor.devices[key] = device
	}
	return device
}

func (governor *Governor) Metrics() Metrics {
	return Metrics{
		Capacity: governor.cpu.capacity, BackgroundCapacity: governor.cpu.limits[2], InteractiveCapacity: governor.cpu.limits[1], LightCapacity: governor.light.capacity,
		ActivePlayback: governor.active[0].Load(), ActiveBackground: governor.active[2].Load(), ActiveInteractive: governor.active[1].Load(),
		WaitingPlayback: governor.waiting[0].Load(), WaitingBackground: governor.waiting[2].Load(), WaitingInteractive: governor.waiting[1].Load(),
	}
}

func (governor *Governor) EncodingCapacity() int {
	if governor == nil {
		return 1
	}
	return governor.cpu.capacity
}

func validClass(class Class) bool {
	return class == Playback || class == Background || class == Interactive
}
