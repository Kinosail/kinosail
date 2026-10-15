package server

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) prepareAttempt(ctx context.Context, item library.Item, recipe hlsRecipe) error { //nolint:cyclop // Final cache, active job, and restart recovery are distinct playback states.
	if manager.cache == "" {
		return errors.New("compatible playback is not configured")
	}
	facts := mediaFactsFor(item, manager.probe.facts(ctx, item))
	resolved, sourceErr := playback.ResolveHLSSource(sharedHLSRecipe(recipe), facts, item.Subtitles)
	if sourceErr != nil {
		return sourceErr
	}
	recipe = localHLSRecipe(resolved)
	key := hlsRecipeKey(item.ID, recipe)
	playlist := filepath.Join(manager.cache, key, "index.m3u8")
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return err
	}
	if startupActualPlayback(ctx) {
		manager.startup.playback(key)
		manager.keepHLSAlive(key, ctx)
	}
	if masterFresh(playlist, item.Path, options.Cache) {
		if pending, err := manager.copiedHLSClockPending(ctx, filepath.Dir(playlist), options.Cache); err == nil && pending {
			if err := manager.ensureCopiedHLSClock(ctx, item, recipe, filepath.Dir(playlist), options.Cache); err != nil {
				return err
			}
		}
	}
	if manager.reusableCopiedHLS(ctx, item, filepath.Dir(playlist), options.Cache, recipe) {
		if err := manager.validateHLSPolicy(ctx, item, recipe, options.Cache); err != nil {
			return err
		}
		return refreshCachedVideoHLSMaster(playlist, facts, recipe, options.Cache)
	}
	job, err := manager.ensureHLSJob(ctx, item, key, options, recipe)
	if err != nil {
		return err
	}

	return manager.waitHLSReadiness(ctx, item, recipe, playlist, options.Cache, job)
}

func (manager *hlsManager) waitHLSReadiness(ctx context.Context, item library.Item, recipe hlsRecipe, playlist, policy string, job *hlsJob) error {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	complete := ctx.Value(startupEncodingKey{}) == nil && manager.shortHEVCNeedsCompletion(ctx, item, recipe)
	ready := func() bool { return manager.hlsPlaybackOutputReady(ctx, item, playlist, policy, complete) }
	for !ready() {
		select {
		case <-ctx.Done():
			return observedHLSReadinessError(ctx.Err(), job)
		case <-job.done:
			if job.err != nil {
				return observedHLSReadinessError(job.err, job)
			}
			if !ready() {
				return errHLSIdentityChanged
			}
		case <-ticker.C:
		}
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return err
	}
	return manager.ensureCopiedHLSClock(ctx, item, recipe, filepath.Dir(playlist), policy)
}

func (manager *hlsManager) hlsPlaybackOutputReady(ctx context.Context, item library.Item, playlist, policy string, complete bool) bool {
	if !masterFresh(playlist, item.Path, policy) {
		return false
	}
	if !complete {
		return true
	}
	return manager.shortHEVCOutputReady(ctx, item, filepath.Dir(playlist), policy)
}
