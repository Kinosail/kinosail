package server

import (
	"context"
	"log/slog"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (probe *mediaProbe) imageTextOptions(language string) mediaprobe.ImageTextOptions {
	return mediaprobe.ImageTextOptions{FFmpeg: probe.ffmpeg, Tesseract: probe.tesseract, CacheDir: probe.cacheDir, Language: language}
}

func (probe *mediaProbe) imageSubtitlesPrepared(ctx context.Context, item library.Item, facts probeResult, language string) bool {
	return len(probe.core.ImageTextCandidates(ctx, item, facts, probe.imageTextOptions(language))) == 0
}

func (probe *mediaProbe) prepareImageSubtitles(ctx context.Context, item library.Item, language string) error {
	options := probe.imageTextOptions(language)
	streams := probe.core.ImageTextCandidates(ctx, item, probe.core.Facts(ctx, item), options)
	if len(streams) == 0 {
		return nil
	}
	started := time.Now()
	if err := probe.core.ConvertImageSubtitles(ctx, item, streams, options); err != nil {
		return err
	}
	slog.InfoContext(ctx, "image subtitles converted to text", "operation", "subtitles.ocr", "item_id", item.ID, "streams", len(streams), "duration_ms", time.Since(started).Milliseconds())
	return nil
}
