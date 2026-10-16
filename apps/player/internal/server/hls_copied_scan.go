package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func watchCopiedHLSAdoption(scan context.Context, preparation *startupEncoding, cancel context.CancelFunc) func() {
	adopted := preparation.adoption()
	done := make(chan struct{})
	go func() {
		select {
		case <-done:
		case <-scan.Done():
		case <-adopted:
			cancel()
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
