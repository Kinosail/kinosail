package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (probe *mediaProbe) embeddedOptions() mediaprobe.EmbeddedOptions {
	return mediaprobe.EmbeddedOptions{FFmpeg: probe.ffmpeg, CacheDir: probe.cacheDir, Enrichment: probe.enrichment()}
}

func (probe *mediaProbe) extractEmbedded(ctx context.Context, item library.Item, stream int) (string, []byte, error) {
	subtitle, err := probe.core.Embedded(ctx, item, stream, probe.embeddedOptions())
	return subtitle.Path, subtitle.Data, err
}
