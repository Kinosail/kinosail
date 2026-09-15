package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (probe *mediaProbe) duration(ctx context.Context, item library.Item) float64 {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Duration(ctx, item)
}
