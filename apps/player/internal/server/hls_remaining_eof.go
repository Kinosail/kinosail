package server

import (
	"bytes"
	"context"
	"github.com/MikeO7/kinosail/packages/library"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Completion is bounded to the demonstrated short source-origin AAC journey.
func remainingColdAACEligible(facts MediaFacts, recipe hlsRecipe, name, method string, duration float64) bool {
	return remainingColdAACRoute(name, method, duration) && facts.Duration == duration &&
		remainingOriginSource(facts, 0) && remainingPlainAudio(recipe) && remainingColdAACRecipe(recipe)
}
func remainingColdAACRoute(name, method string, duration float64) bool {
	return name == "audio/index.m3u8" && method == http.MethodGet && duration > 8.000002 && duration <= 10
}
func remainingColdAACRecipe(recipe hlsRecipe) bool {
	return recipe.offset == 0 && recipe.outputTime == 0 && (recipe.maxBitrate == 0 || recipe.maxBitrate >= 192000)
}

// The caller holds manager.mu; no wait, probe, or filesystem operation occurs here.
func remainingColdAACInitial(job *hlsJob, policy string) (bool, error) {
	if job == nil || job.preparation != nil || job.startNumber != 0 {
		return false, nil
	}
	if job.cachePolicy != policy || job.replacing {
		return false, errHLSIdentityChanged
	}
	if job.done == nil {
		return false, errCopiedHLSIndex
	}
	return true, nil
}

func remainingColdAACWait(ctx context.Context, job *hlsJob) error {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-job.done:
		if err := ctx.Err(); err != nil {
			return err
		}
		return job.err
	}
}

func (manager *hlsManager) remainingColdAACProjection(ctx context.Context, item library.Item, recipe hlsRecipe, key, name, method string, duration float64) (func([]byte) []byte, error) {
	if !remainingColdAACRoute(name, method, duration) {
		return nil, nil
	}
	facts := mediaFactsFor(item, manager.probe.facts(ctx, item))
	if !remainingColdAACEligible(facts, recipe, name, method, duration) {
		return nil, nil
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	manager.mu.Lock()
	job := manager.jobs[key]
	wait, err := remainingColdAACInitial(job, options.Cache)
	manager.mu.Unlock()
	if err != nil || !wait {
		return nil, err
	}
	generation, err := manager.remainingColdAACGeneration(key)
	if err != nil {
		return nil, err
	}
	observed, err := manager.remainingColdAACComplete(ctx, item, recipe, key, options.Cache, job, duration, generation)
	if err != nil {
		return nil, err
	}
	return manager.remainingColdAACProjector(ctx, item, recipe, key, options.Cache, job, duration, generation, observed), nil
}
func (manager *hlsManager) remainingColdAACProjector(ctx context.Context, item library.Item, recipe hlsRecipe, key, policy string, job *hlsJob, duration float64, generation, observed *remainingColdAACState) func([]byte) []byte {
	return func(raw []byte) []byte {
		current, err := manager.remainingColdAACSnapshot(ctx, item, recipe, key, policy, job, duration, generation)
		if err != nil || !observed.same(current) || !bytes.Equal(raw, current.manifest) {
			if err == nil {
				err = errHLSIdentityChanged
			}
			remainingColdAACRejected(ctx, recipe, item, err)
			return nil
		}
		// Use the retained rooted read, not the unchecked pathname bytes.
		return completeHLSVOD(current.manifest, duration)
	}
}

func (manager *hlsManager) remainingColdAACStable(ctx context.Context, item library.Item, recipe hlsRecipe, key, policy string, job *hlsJob) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return err
	}
	manager.mu.Lock()
	current := manager.jobs[key]
	stable := current == nil || current == job && current.cachePolicy == policy && !current.replacing
	manager.mu.Unlock()
	if !stable {
		return errHLSIdentityChanged
	}
	return nil
}

func (manager *hlsManager) remainingColdAACGeneration(key string) (*remainingColdAACState, error) {
	root, err := manager.openCopiedHLSRoot(filepath.Join(manager.cache, key))
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	state := &remainingColdAACState{files: make(map[string]fs.FileInfo)}
	for _, name := range []string{".", "audio"} {
		if err := state.retain(root, name); err != nil {
			return nil, err
		}
	}
	return state, nil
}

func (state *remainingColdAACState) sameGeneration(other *remainingColdAACState) bool {
	if other == nil {
		return false
	}
	for _, name := range []string{".", "audio"} {
		a, b := state.files[name], other.files[name]
		if a == nil || b == nil || !remainingColdAACSameFile(a, b) {
			return false
		}
	}
	return true
}

func (manager *hlsManager) remainingColdAACVerifyGeneration(key string, retained *remainingColdAACState) error {
	canonical, err := manager.remainingColdAACGeneration(key)
	if err != nil {
		return err
	}
	if !retained.sameGeneration(canonical) {
		return errHLSIdentityChanged
	}
	return nil
}

func (manager *hlsManager) remainingColdAACComplete(ctx context.Context, item library.Item, recipe hlsRecipe, key, policy string, job *hlsJob, duration float64, generation *remainingColdAACState) (*remainingColdAACState, error) {
	if err := remainingColdAACWait(ctx, job); err != nil {
		return nil, err
	}
	return manager.remainingColdAACSnapshot(ctx, item, recipe, key, policy, job, duration, generation)
}
