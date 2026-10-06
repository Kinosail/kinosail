package server

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

type startupMetadataKey struct{}

func startupActualPlayback(ctx context.Context) bool {
	return ctx.Value(startupEncodingKey{}) == nil && ctx.Value(startupMetadataKey{}) == nil
}

// Called while holding manager.mu when an actual request joins shared output.
func (manager *hlsManager) adoptStartupJob(ctx context.Context, job *hlsJob, key string) {
	if startupActualPlayback(ctx) && job != nil && job.preparation != nil && !startupJobStopping(job) {
		job.preparation.adopted.Store(true)
		manager.clearStartupCompletion(key)
	}
}

func startupWorkClass(ctx context.Context) workload.Class {
	if ctx.Value(startupEncodingKey{}) != nil {
		return workload.Background
	}
	return workload.Playback
}

func startupInputArguments(ctx context.Context, arguments []string) []string {
	if ctx.Value(startupEncodingKey{}) != nil {
		// Bound speculative read-ahead and CPU. Adoption retains this same stream.
		return append(arguments, "-readrate", "4", "-threads", "2")
	}
	return arguments
}

func startupEncoderThreads(ctx context.Context, threads int) int {
	if ctx.Value(startupEncodingKey{}) != nil {
		return min(2, threads)
	}
	return threads
}

func (manager *hlsManager) watchStartupCancellation(ctx, request context.Context, job *hlsJob) {
	select {
	case <-ctx.Done():
		return
	case <-request.Done():
	}
	manager.mu.Lock()
	if !job.preparation.adopted.Load() {
		job.preparation.stopping = job.preparation.completeVideo
		job.cancel(errHLSInactive)
	}
	manager.mu.Unlock()
}

func (manager *hlsManager) prepareStartupWindow(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) {
	started := time.Now()
	if ctx.Err() != nil {
		return
	}
	preparation.completeVideo = manager.completeHEVCStartup(ctx, item, recipe)
	ctx = context.WithValue(ctx, startupEncodingKey{}, preparation)
	defer func() {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		if !preparation.adopted.Load() {
			manager.startupMarker(hlsRecipeKey(item.ID, recipe), true)
		}
	}()
	state := "unavailable"
	if manager.copiedHLSVideo(ctx, item, recipe) {
		var err error
		preparation.timeline, err = manager.indexCopiedHLS(ctx, item, recipe, preparation)
		if err != nil {
			if preparation.adopted.Load() {
				state = "adopted"
			}
			slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", state, "phase", "copied-video-index", "duration_ms", time.Since(started).Milliseconds())
			return
		}
	}
	if manager.prepare(ctx, item, recipe) == nil {
		state = manager.waitStartupWindow(ctx, item, recipe, preparation)
	}
	slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", state, "duration_ms", time.Since(started).Milliseconds())
}

func (manager *hlsManager) waitStartupWindow(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) string {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	lastSizeCheck := time.Time{}
	for {
		if preparation.adopted.Load() {
			return "adopted"
		}
		if manager.startupWindowReady(item, recipe) {
			return "ready"
		}
		if err := manager.fillStartupWindow(ctx, item, recipe); err != nil {
			return "unavailable"
		}
		if time.Since(lastSizeCheck) >= 250*time.Millisecond {
			lastSizeCheck = time.Now()
			if manager.startupSizeBounded(item, recipe) {
				return "bounded"
			}
		}
		select {
		case <-ctx.Done():
			return "cancelled"
		case <-ticker.C:
		}
	}
}

func (manager *hlsManager) fillStartupWindow(ctx context.Context, item library.Item, recipe hlsRecipe) error {
	if name := manager.missingStartupSegment(item, recipe); name != "" {
		return manager.prepareSegment(ctx, item, recipe, name)
	}
	return nil
}

func (manager *hlsManager) startupSizeBounded(item library.Item, recipe hlsRecipe) bool {
	return startupDirectoryBytes(filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))) >= 64<<20
}

func startupDirectoryBytes(directory string) int64 {
	var size int64
	err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return 64 << 20 // Unknown size cannot grant speculative headroom.
	}
	return size
}

func (manager *hlsManager) prepareDirectRanges(ctx context.Context, item library.Item) {
	manager.probe.facts(ctx, item)
	if ctx.Err() != nil || !manager.index.Safe(item.Path) {
		return
	}
	file, err := os.Open(item.Path) //nolint:gosec // A visible scanned item is revalidated immediately before opening.
	if err != nil {
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	buffer := make([]byte, 64<<10)
	_, _ = file.ReadAt(buffer, 0)
	if ctx.Err() == nil && info.Size() > int64(len(buffer)) {
		_, _ = file.ReadAt(buffer, info.Size()-int64(len(buffer)))
	}
}
