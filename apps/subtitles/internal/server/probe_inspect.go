package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (probe *mediaProbe) inspect(ctx context.Context, item library.Item) probeResult {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Inspect(ctx, item, probe.enrichment())
}

// facts keeps subtitle coverage independent of playback-only enrichment.
func (probe *mediaProbe) facts(ctx context.Context, item library.Item) probeResult {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Facts(ctx, item)
}
