package server

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

type startupMetadataKey struct{}

func startupActualPlayback(ctx context.Context) bool {
	return ctx.Value(startupEncodingKey{}) == nil && ctx.Value(startupMetadataKey{}) == nil
}

// Called while holding manager.mu when an actual request joins shared output.
func adoptStartupJob(ctx context.Context, job *hlsJob) {
	if startupActualPlayback(ctx) && job != nil && job.preparation != nil {
		job.preparation.adopted.Store(true)
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
		job.cancel(errHLSInactive)
	}
	manager.mu.Unlock()
}

func (manager *hlsManager) prepareStartupWindow(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) {
	started := time.Now()
	if ctx.Err() != nil {
		return
	}
	ctx = context.WithValue(ctx, startupEncodingKey{}, preparation)
	defer func() {
		manager.mu.Lock()
		defer manager.mu.Unlock()
		if !preparation.adopted.Load() {
			_ = writeAtomicFile(filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe), ".startup"), []byte("1"))
		}
	}()
	err := manager.prepare(ctx, item, recipe)
	if err != nil {
		slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "unavailable", "duration_ms", time.Since(started).Milliseconds())
		return
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	lastSizeCheck := time.Time{}
	for {
		if manager.startupWindowReady(item, recipe) {
			slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "ready", "duration_ms", time.Since(started).Milliseconds())
			return
		}
		if name := manager.missingStartupSegment(item, recipe); name != "" {
			if err := manager.prepareSegment(ctx, item, recipe, name); err != nil {
				return
			}
		}
		if time.Since(lastSizeCheck) >= 250*time.Millisecond {
			lastSizeCheck = time.Now()
			if startupDirectoryBytes(filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))) >= 64<<20 {
				slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "bounded", "duration_ms", time.Since(started).Milliseconds())
				return
			}
		}
		select {
		case <-ctx.Done():
			slog.InfoContext(ctx, "HLS startup preparation", "request_id", requestActivityID(ctx), "state", "cancelled", "duration_ms", time.Since(started).Milliseconds())
			return
		case <-ticker.C:
		}
	}
}

func (manager *hlsManager) missingStartupSegment(item library.Item, recipe hlsRecipe) string {
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	master, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return ""
	}
	for _, rendition := range strings.Split(string(master), "\n") {
		if !hlsFile(rendition) || !strings.HasSuffix(rendition, "/index.m3u8") {
			continue
		}
		manifest, err := playback.ReadHLSPlaylist(filepath.Join(directory, rendition))
		if err != nil {
			continue
		}
		duration, segmentDuration := 0.0, 0.0
		for _, line := range strings.Split(string(completeHLSVOD(manifest, hlsPlaybackDuration(recipe, manager.probe.duration(manager.ctx, item)))), "\n") {
			if strings.HasPrefix(line, "#EXTINF:") {
				value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
				segmentDuration, _ = strconv.ParseFloat(value, 64)
			}
			if _, valid := hlsSegmentNumber(line); !valid {
				continue
			}
			name := filepath.Join(filepath.Dir(rendition), line)
			if _, err := os.Stat(filepath.Join(directory, name)); os.IsNotExist(err) {
				return name
			}
			duration += segmentDuration
			if duration >= 8 {
				break
			}
		}
	}
	return ""
}

func (manager *hlsManager) startupWindowReady(item library.Item, recipe hlsRecipe) bool {
	if manager.cache == "" || manager.settings == nil {
		return false
	}
	directory := filepath.Join(manager.cache, hlsRecipeKey(item.ID, recipe))
	options, err := manager.seekSettings(item, recipe, directory)
	if err != nil || !masterFresh(filepath.Join(directory, "index.m3u8"), item.Path, options.Cache) {
		return false
	}
	master, err := playback.ReadHLSPlaylist(filepath.Join(directory, "index.m3u8"))
	if err != nil {
		return false
	}
	count := 0
	for _, name := range strings.Split(string(master), "\n") {
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		if !hlsFile(name) || !strings.HasSuffix(name, "/index.m3u8") || !startupRenditionReady(filepath.Join(directory, name), hlsPlaybackDuration(recipe, manager.probe.duration(manager.ctx, item))) {
			return false
		}
		count++
	}
	return count > 0
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

func startupRenditionReady(playlist string, playableDuration float64) bool {
	manifest, err := playback.ReadHLSPlaylist(playlist)
	if err != nil {
		return false
	}
	if info, err := os.Stat(filepath.Join(filepath.Dir(playlist), "init.mp4")); err != nil || info.Size() == 0 {
		return false
	}
	manifest = completeHLSVOD(manifest, playableDuration)
	segments := 0
	duration, segmentDuration := 0.0, 0.0
	for _, name := range strings.Split(string(manifest), "\n") {
		if strings.HasPrefix(name, "#EXTINF:") {
			value, _, _ := strings.Cut(strings.TrimPrefix(name, "#EXTINF:"), ",")
			segmentDuration, _ = strconv.ParseFloat(value, 64)
			if invalidHLSSegmentDuration(segmentDuration) {
				return false
			}
		}
		if _, valid := hlsSegmentNumber(name); !valid {
			continue
		}
		if info, err := os.Stat(filepath.Join(filepath.Dir(playlist), name)); err != nil || info.Size() == 0 {
			return false
		}
		segments++
		duration += segmentDuration
		segmentDuration = 0
		if duration >= 8 {
			return true
		}
	}
	return segments > 0 && playback.PlaylistHas(manifest, "#EXT-X-ENDLIST")
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
