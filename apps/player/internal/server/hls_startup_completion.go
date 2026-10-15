package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
)

const (
	startupCompletionMarker   = ".startup-completion"
	shortHEVCCompletionMarker = ".hevc-complete"
)

func startupJobStopping(job *hlsJob) bool {
	return job != nil && job.preparation != nil &&
		(job.preparation.stopping || job.lifecycle == nil || job.lifecycle.Err() != nil)
}

func (manager *hlsManager) completeHEVCStartup(ctx context.Context, item library.Item, recipe hlsRecipe) bool {
	return recipe.mode == "audio-transcode" && recipe.offset == 0 && recipe.outputTime == 0 &&
		!recipe.dialogueBoost && !recipe.normalizeLoudness &&
		len(recipe.omitted) == 0 && item.Kind != "audio" && item.Kind != "audiobook" &&
		manager.probe.facts(ctx, item).Video.Codec == "hevc"
}

func (manager *hlsManager) shortHEVCNeedsCompletion(ctx context.Context, item library.Item, recipe hlsRecipe) bool {
	duration := manager.probe.duration(ctx, item)
	return duration > 0 && duration <= 30 && manager.completeHEVCStartup(ctx, item, recipe)
}

func (manager *hlsManager) shortHEVCOutputReady(ctx context.Context, item library.Item, directory, policy string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.jobs[filepath.Base(directory)] != nil || ctx.Err() != nil || !cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, policy) {
		return false
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	marker, err := copiedHLSCacheFile(root, shortHEVCCompletionMarker, 16<<10)
	return err == nil && string(marker) == policy && shortHEVCSourceMatches(root, policy)
}

func shortHEVCSourceMatches(root *os.Root, policy string) bool {
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	return err == nil && string(binding) == policy
}

func (manager *hlsManager) publishShortHEVCCompletion(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy string) error {
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return err
	}
	if ctx.Err() != nil || !cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, policy) {
		return errCopiedHLSIndex
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	if !shortHEVCSourceMatches(root, policy) {
		return errCopiedHLSIndex
	}
	return writeCopiedHLSMetadata(root, shortHEVCCompletionMarker, []byte(policy))
}

func (manager *hlsManager) bindStartupCompletion(ctx context.Context, directory, policy string) error {
	preparation, ok := ctx.Value(startupEncodingKey{}).(*startupEncoding)
	if !ok || !preparation.completeVideo {
		return nil
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if ctx.Err() != nil || preparation.stopping {
		return context.Canceled
	}
	if preparation.adopted.Load() {
		return nil
	}
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	return writeStartupCompletion(root, policy)
}

func writeStartupCompletion(root *os.Root, policy string) error {
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	if err != nil || string(binding) != policy {
		return errCopiedHLSIndex
	}
	file, err := root.OpenFile(startupCompletionMarker+".pending", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer func() {
		_ = file.Close()
		_ = root.Remove(startupCompletionMarker + ".pending")
	}()
	_, err = file.Write(binding)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return errCopiedHLSIndex
	}
	return root.Rename(startupCompletionMarker+".pending", startupCompletionMarker)
}

func (manager *hlsManager) startupCompletionReusable(ctx context.Context, item library.Item, directory, policy string) bool {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	if _, err := root.Lstat(startupCompletionMarker); os.IsNotExist(err) {
		return true
	}
	if !startupCompletionMatches(root, policy) {
		return false
	}
	manager.mu.Lock()
	job := manager.jobs[filepath.Base(directory)]
	active := job != nil && job.cachePolicy == policy && job.preparation != nil && !startupJobStopping(job)
	manager.mu.Unlock()
	return ctx.Err() == nil && (active || cacheFresh(filepath.Join(directory, "index.m3u8"), item.Path, policy))
}

func startupCompletionMatches(root *os.Root, policy string) bool {
	marker, err := copiedHLSCacheFile(root, startupCompletionMarker, 16<<10)
	if err != nil || string(marker) != policy {
		return false
	}
	binding, err := copiedHLSCacheFile(root, ".source", 16<<10)
	return err == nil && string(binding) == policy
}

func (manager *hlsManager) clearStartupCompletion(key string) {
	root, err := os.OpenRoot(manager.cache)
	if err != nil {
		return
	}
	defer root.Close()
	_ = root.Remove(filepath.Join(key, startupCompletionMarker))
}
