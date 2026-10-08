package server

import (
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type remainingColdAACState struct {
	manifest []byte
	files    map[string]fs.FileInfo
}

// Bound completion to the demonstrated source-origin AAC journey. Long and
// prepared streams retain their existing startup and lazy-refill paths.
func remainingColdAACEligible(facts MediaFacts, recipe hlsRecipe, name, method string, duration float64) bool {
	return name == "audio/index.m3u8" && method == http.MethodGet &&
		duration > 8.000002 && duration <= 10 && facts.Duration == duration &&
		remainingOriginSource(facts, 0) && remainingPlainAudio(recipe) &&
		recipe.offset == 0 && recipe.outputTime == 0 &&
		(recipe.maxBitrate == 0 || recipe.maxBitrate >= 192000)
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
	if name != "audio/index.m3u8" || method != http.MethodGet || !(duration > 8.000002 && duration <= 10) {
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
	return func(raw []byte) []byte {
		current, err := manager.remainingColdAACSnapshot(ctx, item, recipe, key, options.Cache, job, duration, generation)
		if err != nil || !observed.same(current) || !bytes.Equal(raw, current.manifest) {
			if err == nil {
				err = errHLSIdentityChanged
			}
			remainingColdAACRejected(ctx, recipe, item, err)
			return nil
		}
		// Use the retained rooted read, not the unchecked pathname bytes.
		return completeHLSVOD(current.manifest, duration)
	}, nil
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

func (manager *hlsManager) remainingColdAACSnapshot(ctx context.Context, item library.Item, recipe hlsRecipe, key, policy string, job *hlsJob, duration float64, generation ...*remainingColdAACState) (*remainingColdAACState, error) {
	if err := manager.remainingColdAACStable(ctx, item, recipe, key, policy, job); err != nil {
		return nil, err
	}
	root, err := manager.openCopiedHLSRoot(filepath.Join(manager.cache, key))
	if err != nil {
		return nil, errCopiedHLSIndex
	}
	defer root.Close()
	if _, err := root.Lstat(".copy-timeline"); !os.IsNotExist(err) {
		return nil, errCopiedHLSIndex
	}
	state := &remainingColdAACState{files: make(map[string]fs.FileInfo)}
	for _, name := range []string{".", "audio", ".source", "index.m3u8", "audio/index.m3u8", "audio/init.mp4"} {
		if err := state.retain(root, name); err != nil {
			return nil, err
		}
	}
	if len(generation) > 1 || len(generation) == 1 && !state.sameGeneration(generation[0]) {
		return nil, errHLSIdentityChanged
	}
	if err := manager.remainingColdAACVerifyGeneration(key, state); err != nil {
		return nil, err
	}
	if err := remainingAACReadyRoot(root, policy); err != nil {
		return nil, err
	}
	state.manifest, err = remainingAACManifestRead(root)
	if err != nil || !remainingAACManifestValid(state.manifest) || !playback.PlaylistHas(state.manifest, "#EXT-X-ENDLIST") {
		return nil, errCopiedHLSIndex
	}
	if err := state.retainSegments(ctx, root, duration); err != nil {
		return nil, err
	}
	for name, before := range state.files {
		after, err := root.Lstat(name)
		if err != nil || !remainingColdAACSameFile(before, after) {
			return nil, errHLSIdentityChanged
		}
	}
	if err := manager.remainingColdAACStable(ctx, item, recipe, key, policy, job); err != nil {
		return nil, err
	}
	if err := manager.remainingColdAACVerifyGeneration(key, state); err != nil {
		return nil, err
	}
	return state, nil
}

func (state *remainingColdAACState) retain(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	directory := name == "." || name == "audio"
	if err != nil || directory && !info.IsDir() || !directory && (!info.Mode().IsRegular() || info.Size() <= 0) {
		return errCopiedHLSIndex
	}
	state.files[name] = info
	return nil
}

func (state *remainingColdAACState) retainSegments(ctx context.Context, root *os.Root, duration float64) error {
	count, total := 0, 0.0
	for _, line := range strings.Split(string(state.manifest), "\n") {
		if err := ctx.Err(); err != nil {
			return err
		}
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
			length, err := strconv.ParseFloat(value, 64)
			if err != nil || invalidHLSSegmentDuration(length) {
				return errCopiedHLSIndex
			}
			total += length
		}
		if _, valid := hlsSegmentNumber(line); !valid {
			continue
		}
		count++
		if count > 6 {
			return errCopiedHLSIndex
		}
		name := filepath.Join("audio", line)
		if err := remainingAACExistingAsset(root, name); err != nil {
			return err
		}
		if err := state.retain(root, name); err != nil {
			return err
		}
	}
	if count == 0 || total < duration || total > duration+0.05 {
		return errCopiedHLSIndex
	}
	return nil
}

func remainingColdAACSameFile(a, b fs.FileInfo) bool {
	if a.IsDir() || b.IsDir() {
		return a.IsDir() && b.IsDir() && os.SameFile(a, b)
	}
	return sameCopiedHLSFile(a, b)
}

func (state *remainingColdAACState) same(other *remainingColdAACState) bool {
	if other == nil || !bytes.Equal(state.manifest, other.manifest) || len(state.files) != len(other.files) {
		return false
	}
	for name, before := range state.files {
		after := other.files[name]
		if after == nil || !remainingColdAACSameFile(before, after) {
			return false
		}
	}
	return true
}

// Capture the recipe and rendition identities before waiting for publication.
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
