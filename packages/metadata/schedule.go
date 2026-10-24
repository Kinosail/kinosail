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
	go func() {
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
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-jobs:
				timer := time.NewTimer(scheduleDelay + time.Duration(rand.Int64N(int64(scheduleDelay)))) //nolint:gosec // Refresh jitter only spreads provider load.
				select {
				case <-ctx.Done():
					timer.Stop()
					return
				case <-timer.C:
				}
				select {
				case <-jobs:
				default:
				}
				_ = refresh(ctx)
			}
		}
	}()
	return trigger
}
