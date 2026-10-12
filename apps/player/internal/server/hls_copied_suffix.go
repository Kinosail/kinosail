package server

import (
	"context"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

const maximumCopiedHLSSuffix = 30.0

type copiedHLSRequiredKey struct{}

func copiedHLSIndexRequired(ctx context.Context) bool {
	required, _ := ctx.Value(copiedHLSRequiredKey{}).(bool)
	return required
}

func (manager *hlsManager) coldCopiedHLSNeedsIndex(ctx context.Context, item library.Item, recipe hlsRecipe) bool {
	if recipe.mode != "remux" && recipe.mode != "audio-transcode" || recipe.offset <= 0 || len(recipe.omitted) != 0 || recipe.dialogueBoost || recipe.normalizeLoudness ||
		item.Kind == "audio" || item.Kind == "audiobook" {
		return false
	}
	facts := manager.probe.facts(ctx, item)
	remaining := facts.Duration - recipe.offset
	return facts.Video.Codec == "h264" && remaining > 0 && remaining <= maximumCopiedHLSSuffix && !math.IsNaN(remaining)
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
	root, err := manager.openCopiedHLSRoot(directory)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer root.Close()
	for _, name := range []string{"index.m3u8", ".copy-clock"} {
		if _, err := root.Lstat(name); !os.IsNotExist(err) {
			return errCopiedHLSIndex // A retained output cannot acquire a new origin.
		}
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
	phase := "admission"
	defer func() {
		if result != nil {
			// The existing encoder boundary logs this bounded operation and phase
			// with its owning request/session. No source or process output escapes.
			result = fmt.Errorf("copied HLS suffix %s: %w", phase, result)
		}
	}()
	release, err := manager.workloads.Acquire(ctx, workload.Playback)
	if err != nil {
		return nil, err
	}
	defer release() // Settles before the media encoder acquires its reservation.
	if !manager.index.Safe(item.Path) || manager.validateHLSPolicy(ctx, item, recipe, policy) != nil {
		return nil, errHLSIdentityChanged
	}
	phase = "packets"
	timeline, err = manager.scanCopiedHLSSuffix(ctx, item, policy, recipe.offset, manager.probe.duration(ctx, item))
	if err != nil {
		return nil, err
	}
	phase = "idr"
	if err := manager.certifyCopiedHLS(ctx, item, timeline); err != nil {
		return nil, err
	}
	phase = "configuration"
	if err := manager.certifyCopiedHLSConfiguration(ctx, item, timeline); err != nil {
		return nil, err
	}
	phase = "identity"
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return nil, err
	}
	return timeline, nil
}
