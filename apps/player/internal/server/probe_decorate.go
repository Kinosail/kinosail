package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	"github.com/MikeO7/kinosail/packages/workload"
)

func (probe *mediaProbe) decorate(ctx context.Context, items []library.Item) []library.Item {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Decorate(ctx, items, probe.enrichment())
}

func libraryDecoration(metadata *metadataStore, probe *mediaProbe, background bool) catalog.Decoration {
	return catalog.Decoration{Apply: metadata.apply, Enrich: func(ctx context.Context, items []library.Item) []library.Item {
		return probe.decorate(workload.WithBackgroundPriority(ctx), items)
	}, Enriches: mediaprobe.Decorates, Background: background}
}
