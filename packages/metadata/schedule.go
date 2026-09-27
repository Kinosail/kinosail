package metadata

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

// Schedule queues refreshes after library scans and returns a trigger for configuration changes.
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
		for {
			select {
			case <-ctx.Done():
				return
			case <-jobs:
				_ = refresh(ctx)
			}
		}
	}()
	return trigger
}
