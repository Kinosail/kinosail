package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (probe *mediaProbe) decorate(ctx context.Context, items []library.Item) []library.Item {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Decorate(ctx, items, probe.enrichment())
}
