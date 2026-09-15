package workload

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestEncodingCostIsReservedAtomically(t *testing.T) {
	governor := New(3)
	release, err := governor.AcquireEncoding(t.Context(), Playback, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if next, err := governor.AcquireEncoding(cancelled, Playback, 2, ""); next != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("oversubscribed reservation = %v", err)
	}
	// A blocked/cancelled two-encoder request must not keep the last permit.
	last, err := governor.AcquireEncoding(t.Context(), Playback, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	last()
	release()
	release()
	all, err := governor.AcquireEncoding(t.Context(), Playback, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	all()
}

func TestHardwareSessionBudgetAndInvalidCosts(t *testing.T) { //nolint:cyclop // Admission and rejected costs share the same occupied hardware budget.
	governor := New(3)
	release, err := governor.AcquireEncoding(t.Context(), Playback, 1, "gpu-0")
	if err != nil {
		t.Fatal(err)
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if next, err := governor.AcquireEncoding(cancelled, Background, 1, "gpu-0"); err == nil || next != nil {
		t.Fatal("same GPU was oversubscribed")
	}
	other, err := governor.AcquireEncoding(t.Context(), Playback, 1, "gpu-1")
	if err != nil {
		t.Fatal(err)
	}
	other()
	release()
	for _, cost := range []int{-1, 0, 4} {
		if next, err := governor.AcquireEncoding(t.Context(), Playback, cost, ""); err == nil || next != nil {
			t.Fatalf("accepted invalid cost %d", cost)
		}
	}
	if next, err := governor.AcquireEncoding(t.Context(), Playback, 1, strings.Repeat("x", 513)); err == nil || next != nil {
		t.Fatal("accepted oversized device identity")
	}
	if metrics := governor.Metrics(); metrics.ActivePlayback != 0 || metrics.ActiveBackground != 0 || metrics.WaitingPlayback != 0 || metrics.WaitingBackground != 0 {
		t.Fatalf("reservation leaked: %#v", metrics)
	}
}
