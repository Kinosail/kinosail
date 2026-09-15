package workload

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestGovernorReservesPlaybackCapacity(t *testing.T) { //nolint:cyclop // The score of 13 remains below the repository ceiling of 22 for the capacity matrix.
	governor := New(3)
	releaseBackground, err := governor.Acquire(t.Context(), Background)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseBackground()

	playback, cancelPlayback := context.WithCancel(t.Context())
	releasePlayback, err := governor.Acquire(playback, Playback)
	if err != nil {
		t.Fatal(err)
	}
	defer releasePlayback()
	cancelPlayback()

	queued, cancelQueued := context.WithCancel(t.Context())
	cancelQueued()
	if release, acquireErr := governor.Acquire(queued, Background); acquireErr == nil || release != nil {
		t.Fatalf("canceled background acquire returned release=%t, error=%v", release != nil, acquireErr)
	}

	metrics := governor.Metrics()
	if metrics.Capacity != 3 || metrics.BackgroundCapacity != 1 || metrics.ActivePlayback != 1 || metrics.ActiveBackground != 1 || metrics.WaitingPlayback != 0 || metrics.WaitingBackground != 0 {
		t.Fatalf("metrics = %#v", metrics)
	}

	releasePlayback()
	releasePlayback()
	releaseBackground()
	metrics = governor.Metrics()
	if metrics.ActivePlayback != 0 || metrics.ActiveBackground != 0 {
		t.Fatalf("released metrics = %#v", metrics)
	}
}

func TestGovernorCancellationReleasesBackgroundSlot(t *testing.T) {
	governor := New(1)
	release, err := governor.Acquire(t.Context(), Playback)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	result := make(chan struct {
		release func()
		err     error
	}, 1)
	go func() {
		acquired, acquireErr := governor.Acquire(ctx, Background)
		result <- struct {
			release func()
			err     error
		}{acquired, acquireErr}
	}()
	for deadline := time.Now().Add(time.Second); len(governor.background) == 0 && time.Now().Before(deadline); runtime.Gosched() {
	}
	if len(governor.background) != 1 {
		t.Fatal("background acquire did not reach total capacity")
	}
	cancel()
	acquired := <-result
	if acquired.err == nil || acquired.release != nil {
		t.Fatalf("canceled acquire returned release=%t, error=%v", acquired.release != nil, acquired.err)
	}
	if len(governor.background) != 0 {
		t.Fatal("canceled acquire retained background capacity")
	}
	release()

	background, err := governor.Acquire(t.Context(), Background)
	if err != nil {
		t.Fatal(err)
	}
	background()
}

func TestGovernorReservesHeavyCapacityForPlayback(t *testing.T) {
	governor := New(2)
	releaseBackground, err := governor.Acquire(t.Context(), Background)
	if err != nil {
		t.Fatal(err)
	}
	playback, cancelPlayback := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancelPlayback()
	releasePlayback, err := governor.Acquire(playback, Playback)
	if err != nil {
		t.Fatalf("playback waited behind background work: %v", err)
	}

	queued, cancelQueued := context.WithCancel(t.Context())
	result := make(chan error, 1)
	go func() {
		release, acquireErr := governor.Acquire(queued, Background)
		if release != nil {
			release()
		}
		result <- acquireErr
	}()
	select {
	case acquireErr := <-result:
		t.Fatalf("second background job was admitted: %v", acquireErr)
	case <-time.After(25 * time.Millisecond):
	}
	cancelQueued()
	if acquireErr := <-result; !errors.Is(acquireErr, context.Canceled) {
		t.Fatalf("queued cancellation = %v", acquireErr)
	}
	releasePlayback()
	releaseBackground()
}

func TestGovernorReservesTwoHeavySlotsForPlayback(t *testing.T) {
	if runtime.GOMAXPROCS(0) < 4 {
		t.Skip("requires enough processors for two playback slots")
	}
	governor := New(HeavyCapacity())
	releaseBackground, err := governor.Acquire(t.Context(), Background)
	if err != nil {
		t.Fatal(err)
	}
	defer releaseBackground()
	for range 2 {
		playback, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
		release, acquireErr := governor.Acquire(playback, Playback)
		cancel()
		if acquireErr != nil {
			t.Fatalf("playback waited behind one background job: %v", acquireErr)
		}
		defer release()
	}
}

func TestNilGovernorAndMinimumCapacity(t *testing.T) {
	var governor *Governor
	release, err := governor.Acquire(t.Context(), Playback)
	if err != nil || release == nil {
		t.Fatalf("nil acquire returned release=%t, error=%v", release != nil, err)
	}
	release()

	metrics := New(0).Metrics()
	if metrics.Capacity != 1 || metrics.BackgroundCapacity != 1 {
		t.Fatalf("minimum metrics = %#v", metrics)
	}
}

func TestHeavyCapacityTracksAvailableProcessors(t *testing.T) {
	want := min(3, max(1, runtime.GOMAXPROCS(0)-1))
	if got := HeavyCapacity(); got != want {
		t.Fatalf("HeavyCapacity() = %d, want %d", got, want)
	}
}

func BenchmarkPlaybackAdmissionWhileBackgroundWorkRuns(b *testing.B) {
	governor := New(2)
	releaseBackground, err := governor.Acquire(b.Context(), Background)
	if err != nil {
		b.Fatal(err)
	}
	defer releaseBackground()
	b.ReportAllocs()
	for b.Loop() {
		release, acquireErr := governor.Acquire(b.Context(), Playback)
		if acquireErr != nil {
			b.Fatal(acquireErr)
		}
		release()
	}
}
