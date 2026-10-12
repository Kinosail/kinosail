package server

import (
	"context"
	"errors"
	"log/slog"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/workload"
)

const maximumCopiedHLSKeys = 4096

var errCopiedHLSIndex = errors.New("copied HLS timeline is unavailable")

type copiedHLSKey = mediaprobe.H264Key

type copiedHLSTimeline struct {
	Policy      string
	Strategy    string
	Numerator   int64
	Denominator int64
	TimeBase    float64
	Keys        []copiedHLSKey
	End         float64
	Clock       *float64
	AudioGrid   *copiedHLSAudioGrid   `json:",omitempty"`
	AudioSource *copiedHLSSourceAudio `json:",omitempty"`
}

func (timeline *copiedHLSTimeline) point(number int) float64 {
	return float64(timeline.Keys[number].PTS) * timeline.TimeBase
}

func (manager *hlsManager) indexCopiedHLS(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) (*copiedHLSTimeline, error) {
	phase := "eligibility"
	defer func() {
		slog.DebugContext(ctx, "HLS copied index", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "phase", phase, "canceled", ctx.Err() != nil)
	}()
	facts := manager.probe.facts(ctx, item)
	if !manager.copiedHLSVideo(ctx, item, recipe) || facts.Video.Codec != "h264" || len(recipe.omitted) != 0 || !manager.index.Safe(item.Path) {
		return nil, errCopiedHLSIndex
	}
	phase = "settings"
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	scan, cancel := context.WithCancel(ctx)
	defer cancel()
	stopWatching := watchCopiedHLSAdoption(scan, preparation, cancel)
	defer stopWatching()

	phase = "admission"
	release, err := manager.workloads.Acquire(scan, workload.Background)
	if err != nil {
		return nil, err
	}
	defer release()

	phase = "video-index"
	timeline, err := manager.scanCopiedHLS(scan, item, options.Cache, 0, facts.Duration, facts.Duration+0.1, 128<<20, 2_000_010)
	if err != nil {
		return nil, err
	}
	phase = "selection"
	selected, err := manager.selectCopiedHLSTimeline(scan, item, recipe, preparation, timeline)
	if err != nil {
		return nil, err
	}
	phase = "audio-clock"
	if err := manager.indexCopiedHLSAudio(scan, item, recipe, selected); err != nil {
		return nil, err
	}
	return selected, nil
}

func copiedHLSLines(ctx context.Context, executable string, arguments []string, maximumBytes int64, maximumLines int, visit func(string) error) error {
	logger := slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx))
	if err := mediaprobe.ProbeLines(ctx, logger, executable, arguments, maximumBytes, maximumLines, visit); err != nil {
		return errCopiedHLSIndex
	}
	return nil
}
