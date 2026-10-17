package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (probe *mediaProbe) inspect(ctx context.Context, item library.Item) probeResult {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.WithConvertedSubtitles(item, probe.core.Inspect(ctx, item, probe.enrichment()), probe.cacheDir)
}

func (probe *mediaProbe) cachedFacts(item library.Item) (probeResult, bool) {
	if probe.executable == "" {
		return probeResult{}, false
	}
	probe.core.ConfigureCache(probe.cacheDir)
	result, found := probe.core.CachedFacts(item)
	if found && probe.markers != nil {
		result.Markers = probe.markers.Markers(item, result.Markers)
	}
	return probe.core.WithConvertedSubtitles(item, result, probe.cacheDir), found
}

func (probe *mediaProbe) facts(ctx context.Context, item library.Item) probeResult {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.WithConvertedSubtitles(item, probe.core.Facts(ctx, item), probe.cacheDir)
}
