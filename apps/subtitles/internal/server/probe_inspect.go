package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
)

func (probe *mediaProbe) facts(ctx context.Context, item library.Item) probeResult {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Facts(ctx, item)
}
