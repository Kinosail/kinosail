package metadata

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

const scheduleDelay = 250 * time.Millisecond

var healInterval = time.Hour

func Schedule(ctx context.Context, available bool, observe func(func([]library.Item)), refresh func(context.Context) error) func() {
	if ctx == nil || !available || observe == nil || refresh == nil {
		return nil
	}
	jobs := make(chan struct{}, 1)
	trigger := func() {
		select {
		case jobs <- struct{}{}:
		default:
		}
	}
	observe(func([]library.Item) { trigger() })
	go healPeriodically(ctx, trigger)
	go refreshScheduled(ctx, jobs, refresh)
	return trigger
}

func healPeriodically(ctx context.Context, trigger func()) {
	ticker := time.NewTicker(healInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			trigger()
		}
	}
}

func refreshScheduled(ctx context.Context, jobs chan struct{}, refresh func(context.Context) error) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-jobs:
			if !waitRefreshJitter(ctx) {
				return
			}
			select {
			case <-jobs:
			default:
			}
			_ = refresh(ctx)
		}
	}
}

func waitRefreshJitter(ctx context.Context) bool {
	timer := time.NewTimer(scheduleDelay + time.Duration(rand.Int64N(int64(scheduleDelay)))) //nolint:gosec // Refresh jitter only spreads provider load.
	select {
	case <-ctx.Done():
		timer.Stop()
		return false
	case <-timer.C:
		return true
	}
}
