package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

type copiedAACWorkerKey struct{}

type copiedAACWorkerIdentity struct {
	key    string
	recipe hlsRecipe
	policy string
	job    *hlsJob
}

func copiedAACCanonicalRecipe(ctx context.Context, recipe hlsRecipe, policy string) hlsRecipe {
	identity, ok := ctx.Value(copiedAACWorkerKey{}).(*copiedAACWorkerIdentity)
	if ok && identity != nil && identity.policy == policy && copiedAACPolicyRequired(policy) {
		return identity.recipe
	}
	return recipe
}

// Called only under hls.mu; it performs no source I/O, subprocess or wait.
func (manager *hlsManager) copiedAACWorkerCurrentLocked(ctx context.Context, key, policy string) bool {
	identity, ok := ctx.Value(copiedAACWorkerKey{}).(*copiedAACWorkerIdentity)
	return ok && validCopiedAACWorkerIdentity(identity, key, policy) && len(manager.jobs) == 1 &&
		manager.jobs[key] == identity.job && identity.job.lifecycle.Err() == nil && ctx.Err() == nil &&
		identity.job.observation == hlsObservationFor(ctx)
}

func (manager *hlsManager) copiedAACWorkerCurrent(ctx context.Context, key, policy string) bool {
	manager.mu.Lock()
	current := manager.copiedAACWorkerCurrentLocked(ctx, key, policy)
	manager.mu.Unlock()
	return current
}

// Foreground migration reindexes before reserving an encoding slot. Only this
// exact observed live job may use the exception to the usual idle index rule.
func (manager *hlsManager) bindCopiedAACIndex(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy string, number int) error {
	if !copiedAACPolicyRequired(policy) {
		return nil
	}
	if handled, err := copiedAACPreparedIndex(ctx, policy); handled {
		return err
	}
	if manager.copiedHLSTimelinePresent(directory) {
		timeline, err := manager.readCopiedHLSTimelineContext(ctx, directory, policy)
		if err != nil || timeline.AudioOrigin == nil {
			return errCopiedHLSIndex
		}
		return nil
	}
	recipe = copiedAACCanonicalRecipe(ctx, recipe, policy)
	key := hlsRecipeKey(item.ID, recipe)
	if number != 0 || !manager.copiedAACWorkerCurrent(ctx, key, policy) {
		return errCopiedHLSIndex // A selected producer never falls back to unindexed encoding.
	}
	return manager.writeCopiedAACWorkerIndex(ctx, item, recipe, directory, policy, key)
}

func validCopiedAACWorkerIdentity(identity *copiedAACWorkerIdentity, key, policy string) bool {
	return identity != nil && identity.key == key && identity.policy == policy && copiedAACPolicyRequired(policy) &&
		identity.job != nil && identity.job.cachePolicy == policy && identity.job.startNumber == 0
}

func copiedAACPreparedIndex(ctx context.Context, policy string) (bool, error) {
	preparation, ok := ctx.Value(startupEncodingKey{}).(*startupEncoding)
	if !ok || preparation.timeline == nil {
		return false, nil
	}
	if preparation.timeline.AudioOrigin == nil || preparation.timeline.Policy != policy {
		return true, errCopiedHLSIndex
	}
	return true, nil
}

func (manager *hlsManager) writeCopiedAACWorkerIndex(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy, key string) error {
	timeline, err := manager.indexCopiedHLS(ctx, item, recipe, &startupEncoding{})
	if err != nil || timeline.AudioOrigin == nil || !manager.copiedAACWorkerCurrent(ctx, key, policy) ||
		manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return errCopiedHLSIndex
	}
	if err := manager.writeCopiedHLSTimeline(directory, timeline); err != nil {
		return err
	}
	if !manager.copiedAACWorkerCurrent(ctx, key, policy) {
		return errCopiedHLSIndex
	}
	return nil
}
