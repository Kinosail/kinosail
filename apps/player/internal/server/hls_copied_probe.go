package server

import (
	"context"
	"log/slog"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (manager *hlsManager) scanCopiedHLS(ctx context.Context, item library.Item, policy string, from, duration, span float64, maximumBytes int64, maximumLines int) (*copiedHLSTimeline, error) {
	index, err := manager.probe.core.H264Index(ctx, manager.ffmpeg, mediaprobe.H264Scan{
		Path: item.Path, From: from, Duration: duration, MaximumSpan: span,
		MaximumPacketBytes: maximumBytes, MaximumPacketLines: maximumLines,
		Logger: slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx)),
	})
	if err != nil {
		return nil, err
	}
	numerator, denominator := index.TimeBase()
	return &copiedHLSTimeline{
		Policy: policy, Strategy: "h264-idr-keys-1", Numerator: numerator, Denominator: denominator,
		TimeBase: float64(numerator) / float64(denominator), Keys: index.Keys(), End: index.End(),
	}, nil
}
