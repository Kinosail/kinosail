package server

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

type copiedHLSRequiredKey struct{}

func copiedHLSIndexRequired(ctx context.Context) bool {
	required, _ := ctx.Value(copiedHLSRequiredKey{}).(bool)
	return required
}

func (manager *hlsManager) coldCopiedHLSNeedsIndex(ctx context.Context, item library.Item, recipe hlsRecipe) bool {
	if !coldCopiedHLSRecipe(item, recipe) {
		return false
	}
	facts := manager.probe.facts(ctx, item)
	remaining := facts.Duration - recipe.offset
	return facts.Video.Codec == "h264" && remaining > 0 &&
		(recipe.mode == "remux" || remaining <= maximumCopiedHLSAudioReplay)
}

func coldCopiedHLSRecipe(item library.Item, recipe hlsRecipe) bool {
	return !(recipe.mode != "remux" && recipe.mode != "audio-transcode" || recipe.offset <= 0 || len(recipe.omitted) != 0 || recipe.dialogueBoost || recipe.normalizeLoudness ||
		item.Kind == "audio" || item.Kind == "audiobook")
}

func (manager *hlsManager) bindCopiedHLSIndex(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy string, number int) error {
	if ctx.Value(startupEncodingKey{}) != nil {
		return manager.bindCopiedHLSTimeline(ctx, directory, number)
	}
	if manager.copiedHLSTimelinePresent(directory) {
		// Refills retain the original source cuts and mux clock. Never replace
		// a present certificate with a newly scanned view of the same path.
		_, err := manager.readCopiedHLSTimelineContext(ctx, directory, policy)
		return err
	}
	if copiedHLSIndexRequired(ctx) {
		return errCopiedHLSIndex
	}
	if !manager.coldCopiedHLSNeedsIndex(ctx, item, recipe) {
		return nil
	}
	if number != 0 {
		return errCopiedHLSIndex
	}
	if !manager.copiedHLSCacheUnwritten(directory) {
		return errCopiedHLSIndex
	}
	timeline, err := manager.indexCopiedHLSSuffix(ctx, item, recipe, policy)
	if err != nil {
		return err
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return err
	}
	if err := manager.writeCopiedHLSTimeline(directory, timeline); err != nil {
		return err
	}
	return manager.validateHLSPolicy(ctx, item, recipe, policy)
}

func (manager *hlsManager) indexCopiedHLSSuffix(parent context.Context, item library.Item, recipe hlsRecipe, policy string) (timeline *copiedHLSTimeline, result error) {
	// Acquisition and all three probes share one deadline. The encoding job
	// owns this work; requests reuse the job instead of starting more probes.
	ctx, cancel := context.WithTimeout(parent, 8*time.Second)
	defer cancel()
	phase := "duration"
	defer func() {
		if result != nil {
			// The existing encoder boundary logs this bounded operation and phase
			// with its owning request/session. No source or process output escapes.
			result = fmt.Errorf("copied HLS suffix %s: %w", phase, result)
		}
	}()
	duration := manager.probe.duration(ctx, item)
	if !validCopiedHLSEnd(duration) {
		return nil, errCopiedHLSIndex
	}
	phase = "admission"
	release, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		return nil, err
	}
	defer release() // Settles before the media encoder acquires its reservation.
	if !manager.index.Safe(item.Path) || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return nil, errHLSIdentityChanged
	}
	phase = "video-index"
	timeline, err = manager.scanCopiedHLS(ctx, item, policy, recipe.offset, duration, copiedHLSSuffixSpan(duration, recipe.mode), 8<<20, 100_010)
	if err != nil {
		return nil, err
	}
	phase = "identity"
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return nil, err
	}
	return timeline, nil
}

func (manager *hlsManager) copiedHLSCacheUnwritten(directory string) bool {
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return false
	}
	defer root.Close()
	for _, name := range []string{"index.m3u8", ".copy-clock"} {
		if _, err := root.Lstat(name); !os.IsNotExist(err) {
			return false // A retained output cannot acquire a new origin.
		}
	}
	return true
}

func copiedHLSSuffixSpan(duration float64, mode string) float64 {
	if mode == "audio-transcode" {
		return maximumCopiedHLSAudioReplay
	}
	// Remux scans retain the format/video EOF tolerance within the shared
	// deadline and output limits. Audio replay keeps its encoder-history bound.
	return duration + 0.1
}
