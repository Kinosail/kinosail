package server

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/workload"
)

func (manager *hlsManager) createCopiedHLSPlan(ctx context.Context, item library.Item, recipe hlsRecipe, directory, policy string) (*copiedHLSPlan, error) {
	if !copiedHLSMode(recipe) {
		return nil, nil
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return nil, err
	}
	plan, err := manager.selectCopiedHLSPlan(ctx, item, recipe, policy)
	if err != nil {
		return nil, err
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return nil, err
	}
	data, err := json.Marshal(plan)
	if err != nil || !plan.valid(policy) {
		return nil, errCopiedHLSPlan
	}
	if err := writeAtomicFile(filepath.Join(directory, copiedHLSPlanFile), data); err != nil {
		return nil, err
	}
	if err := manager.validateHLSPolicy(ctx, item, recipe, policy); err != nil {
		return nil, err
	}
	return plan, nil
}

func (manager *hlsManager) selectCopiedHLSPlan(ctx context.Context, item library.Item, recipe hlsRecipe, policy string) (*copiedHLSPlan, error) {
	plan := &copiedHLSPlan{Version: 1, Policy: policy, Strategy: "legacy"}
	facts := mediaFactsFor(item, manager.probe.inspect(ctx, item))
	if completedCopiedHLSWindow(recipe, facts.Duration) {
		plan.Strategy = "completed"
		return plan, nil
	}
	if recipe.offset == 0 && len(recipe.omitted) == 0 && facts.Video.Codec == "h264" && manager.index.Safe(item.Path) {
		index, err := manager.scanCopiedHLSPlan(ctx, item, facts.Duration)
		if ctx.Err() != nil || errors.Is(err, mediaprobe.ErrProbeLifecycle) {
			return nil, errors.Join(ctx.Err(), err)
		}
		if err == nil {
			plan.certify(index)
		}
		if plan.Strategy == "legacy" {
			slog.WarnContext(ctx, "HLS copied index unavailable", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "outcome", "legacy-before-encoding")
		}
	}
	return plan, nil
}

func (manager *hlsManager) scanCopiedHLSPlan(ctx context.Context, item library.Item, duration float64) (mediaprobe.H264Index, error) {
	scan, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	release, err := manager.workloads.Acquire(scan, workload.Background)
	if err != nil {
		return mediaprobe.H264Index{}, err
	}
	defer release()
	return manager.probe.core.H264Index(scan, manager.ffmpeg, mediaprobe.H264Scan{
		Path:     item.Path,
		Duration: duration, MaximumSpan: duration + 0.1, MaximumPacketBytes: 128 << 20, MaximumPacketLines: 2_000_010,
		Logger: slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx)),
	})
}

func (plan *copiedHLSPlan) certify(index mediaprobe.H264Index) {
	keys := index.Keys()
	if len(keys) == 0 || len(keys) >= maxHLSPlaylistURIs || keys[0].PTS != 0 {
		return
	}
	numerator, denominator := index.TimeBase()
	cuts := make([]float64, 0, len(keys)+1)
	for _, key := range keys {
		cuts = append(cuts, float64(key.PTS)*float64(numerator)/float64(denominator))
	}
	cuts = append(cuts, index.End())
	plan.Strategy, plan.Cuts = "certified", cuts
}

func (plan *copiedHLSPlan) finalizeOutput(directory, name, playlist string) error {
	if err := finalizePlaylist(playlist); err != nil {
		return err
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer root.Close()
	return plan.validateRendition(root, filepath.Join(name, "index.m3u8"))
}

func (plan *copiedHLSPlan) segmentMode(mode string) string {
	if plan != nil && plan.Strategy == "certified" {
		return "copied-keyframes"
	}
	return mode
}

// A short copied suffix can retain earlier packets than its requested offset.
// Publish its actual completed map instead of extrapolating a shorter timeline.
func completedCopiedHLSWindow(recipe hlsRecipe, duration float64) bool {
	remaining := playback.HLSPlaybackDuration(sharedHLSRecipe(recipe), duration)
	return copiedHLSMode(recipe) && recipe.offset > 0 && remaining > 0 && remaining <= 30
}
