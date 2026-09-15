package metadata

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

// Schedule queues one metadata refresh after each completed library scan.
func Schedule(ctx context.Context, available bool, observe func(func([]library.Item)), refresh func(context.Context) error) {
	if ctx == nil || !available || observe == nil || refresh == nil {
		return
	}
	jobs := make(chan struct{}, 1)
	observe(func([]library.Item) {
		select {
		case jobs <- struct{}{}:
		default:
		}
	})
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
}
