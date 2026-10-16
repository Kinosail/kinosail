package workload

import (
	"context"
	"errors"
	"testing"
	"time"
)

func admitted(t *testing.T, acquire func(context.Context) (func(), error)) func() {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	release, err := acquire(ctx)
	if err != nil {
		t.Fatalf("reservation was not admitted: %v", err)
	}
	return release
}

func blocked(t *testing.T, acquire func(context.Context) (func(), error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
	defer cancel()
	release, err := acquire(ctx)
	if err == nil {
		release()
		t.Fatal("reservation was admitted beyond its budget")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked reservation error = %v", err)
	}
}

func waitFor(t *testing.T, condition func(Metrics) bool, governor *Governor) {
	t.Helper()
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(time.Millisecond) {
		if condition(governor.Metrics()) {
			return
		}
	}
	t.Fatalf("metrics never reached the expected state: %#v", governor.Metrics())
}

func TestStreamCopyIsNotBlockedByAFullCPULane(t *testing.T) {
	governor := New(2)
	encode := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 2, "") })
	defer encode()
	blocked(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 1, "") })
	for range 4 {
		release := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireLight(ctx, Playback) })
		defer release()
	}
	background := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireLight(ctx, Background) })
	background()
}

func TestTwoPlaybacksShareOneGPU(t *testing.T) {
	governor := New(1)
	cpu := admitted(t, func(ctx context.Context) (func(), error) { return governor.Acquire(ctx, Playback) })
	defer cpu()
	for _, device := range []string{"/dev/dri/renderD128", "cuda:0"} {
		first := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 1, device) })
		second := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 1, device) })
		first()
		second()
	}
}

func TestQueuedGPUEncodesDoNotHoldStreamCopySlots(t *testing.T) {
	governor := New(4, DeviceSessions(func(string) int { return 2 }))
	const device = "vaapi:/dev/dri/renderD128"
	acquireGPU := func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 1, device) }
	acquireLight := func(ctx context.Context) (func(), error) { return governor.AcquireLight(ctx, Playback) }
	encoding := []func(){admitted(t, acquireGPU), admitted(t, acquireGPU)}
	queued, cancel := context.WithCancel(t.Context())
	defer cancel()
	failures := make(chan error, 8)
	for range 8 {
		go func() {
			release, err := acquireGPU(queued)
			if err == nil {
				release()
			}
			failures <- err
		}()
	}
	waitFor(t, func(metrics Metrics) bool { return metrics.WaitingPlayback == 8 }, governor)
	streams := make([]func(), 0, governor.Metrics().LightCapacity)
	for range governor.Metrics().LightCapacity - len(encoding) {
		streams = append(streams, admitted(t, acquireLight))
	}
	blocked(t, acquireLight)
	cancel()
	for range 8 {
		if err := <-failures; !errors.Is(err, context.Canceled) {
			t.Fatalf("queued GPU encode error = %v", err)
		}
	}
	encoding[0]()
	streams = append(streams, admitted(t, acquireLight))
	blocked(t, acquireGPU)
	for _, release := range streams {
		release()
	}
	replacement := admitted(t, acquireGPU)
	replacement()
	encoding[1]()
	if metrics := governor.Metrics(); metrics.ActivePlayback != 0 || metrics.WaitingPlayback != 0 {
		t.Fatalf("reservations leaked: %#v", metrics)
	}
	streams = streams[:0]
	for range governor.Metrics().LightCapacity {
		streams = append(streams, admitted(t, acquireLight))
	}
	for _, release := range streams {
		release()
	}
}

func TestBackgroundNeverHoldsTheLastPlaybackGPUSlot(t *testing.T) {
	governor := New(8, DeviceSessions(func(string) int { return 3 }))
	const device = "/dev/dri/renderD128"
	acquireBackground := func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Background, 1, device) }
	acquirePlayback := func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 1, device) }
	first, second := admitted(t, acquireBackground), admitted(t, acquireBackground)
	blocked(t, acquireBackground)
	playback := admitted(t, acquirePlayback)
	blocked(t, acquirePlayback)
	blocked(t, acquireBackground)
	for _, release := range []func(){first, second, playback} {
		release()
	}
	for range 3 {
		release := admitted(t, acquirePlayback)
		defer release()
	}
	blocked(t, acquireBackground)
	if _, err := New(8, DeviceSessions(func(string) int { return 1 })).AcquireEncoding(t.Context(), Background, 1, device); err == nil {
		t.Fatal("background work held the only GPU session")
	}
}

func TestCostThreeWaiterIsAdmittedBeforeLaterSmallWork(t *testing.T) {
	governor := New(4)
	large := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 2, "") })
	small := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 1, "") })
	order := make(chan int, 2)
	releases := make(chan func(), 2)
	request := func(cost int) {
		release, err := governor.AcquireEncoding(t.Context(), Playback, cost, "")
		if err != nil {
			t.Error(err)
			order <- 0
			return
		}
		order <- cost
		releases <- release
	}
	go request(3)
	waitFor(t, func(metrics Metrics) bool { return metrics.WaitingPlayback == 1 }, governor)
	go request(1)
	waitFor(t, func(metrics Metrics) bool { return metrics.WaitingPlayback == 2 }, governor)
	time.Sleep(20 * time.Millisecond)
	if len(order) != 0 {
		t.Fatalf("later cost-1 work overtook the queued cost-3 waiter: %d", <-order)
	}
	large()
	if first := <-order; first != 3 {
		t.Fatalf("first admitted cost = %d, want 3", first)
	}
	small()
	if second := <-order; second != 1 {
		t.Fatalf("second admitted cost = %d, want 1", second)
	}
	(<-releases)()
	(<-releases)()
	if metrics := governor.Metrics(); metrics.ActivePlayback != 0 || metrics.WaitingPlayback != 0 {
		t.Fatalf("reservations leaked: %#v", metrics)
	}
}

func TestInteractiveWorkIsNotQueuedBehindBackgroundWork(t *testing.T) {
	governor := New(4)
	held := []func(){}
	for range governor.Metrics().BackgroundCapacity {
		held = append(held, admitted(t, func(ctx context.Context) (func(), error) { return governor.Acquire(ctx, Background) }))
	}
	blocked(t, func(ctx context.Context) (func(), error) { return governor.Acquire(ctx, Background) })
	interactive := admitted(t, func(ctx context.Context) (func(), error) { return governor.Acquire(ctx, Interactive) })
	if metrics := governor.Metrics(); metrics.ActiveInteractive != 1 || metrics.InteractiveCapacity < 1 {
		t.Fatalf("interactive metrics = %#v", metrics)
	}
	interactive()
	for _, release := range held {
		release()
	}
}

func TestPlaybackIsAdmittedWhileBackgroundUsesIdleCPU(t *testing.T) {
	governor := New(4)
	background := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Background, 2, "") })
	defer background()
	playback := admitted(t, func(ctx context.Context) (func(), error) { return governor.AcquireEncoding(ctx, Playback, 4, "") })
	blocked(t, func(ctx context.Context) (func(), error) { return governor.Acquire(ctx, Background) })
	playback()
}

func TestCPUCapacityScalesWithProcessors(t *testing.T) {
	for _, test := range []struct{ processors, capacity, background int }{{1, 1, 1}, {2, 2, 1}, {4, 4, 2}, {16, 16, 8}, {64, 64, 32}} {
		metrics := New(capacityFor(test.processors)).Metrics()
		if metrics.Capacity != test.capacity || metrics.BackgroundCapacity != test.background || metrics.LightCapacity < 8 {
			t.Errorf("%d processors: metrics = %#v", test.processors, metrics)
		}
	}
}

func TestLadderCostFollowsOutputPixels(t *testing.T) {
	ladder := []Rung{{640, 360, 24}, {768, 432, 24}, {960, 540, 24}, {1280, 720, 24}, {1920, 1080, 24}}
	for _, test := range []struct{ capacity, rungs, cost int }{{1, 1, 1}, {2, 1, 2}, {4, 1, 2}, {6, 4, 3}, {8, 5, 4}, {32, 5, 4}} {
		governor := New(test.capacity)
		rungs := governor.LadderRungs(ladder)
		if rungs != test.rungs {
			t.Errorf("capacity %d: rungs = %d, want %d", test.capacity, rungs, test.rungs)
		}
		if cost := governor.LadderCost(ladder[len(ladder)-rungs:]); cost != test.cost {
			t.Errorf("capacity %d: cost = %d, want %d", test.capacity, cost, test.cost)
		}
	}
	if cost := New(16).LadderCost([]Rung{{3840, 2160, 60}}); cost != 16 {
		t.Errorf("4K60 cost = %d, want 16", cost)
	}
	if cost := New(16).LadderCost([]Rung{{1920, 1080, 0}}); cost != 2 {
		t.Errorf("unknown frame rate cost = %d, want 2", cost)
	}
}
