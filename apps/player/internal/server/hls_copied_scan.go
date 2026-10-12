package server

import (
	"context"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func watchCopiedHLSAdoption(scan context.Context, preparation *startupEncoding, cancel context.CancelFunc) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-scan.Done():
				return
			case <-ticker.C:
				if preparation.adopted.Load() {
					cancel()
					return
				}
			}
		}
	}()
	return func() { close(done) }
}

func (manager *hlsManager) selectCopiedHLSTimeline(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding, timeline *copiedHLSTimeline) (*copiedHLSTimeline, error) {
	first := -1
	for number := range timeline.Keys {
		if timeline.point(number) > recipe.offset+0.000001 {
			break
		}
		first = number
	}
	if first < 0 {
		return nil, errCopiedHLSIndex
	}
	timeline.Keys = timeline.Keys[first:]
	if preparation.adopted.Load() || manager.validateHLSPolicy(ctx, item, recipe, timeline.Policy) != nil {
		return nil, errCopiedHLSIndex
	}
	manager.mu.Lock()
	idle := len(manager.jobs) == 0
	manager.mu.Unlock()
	if !idle {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}
