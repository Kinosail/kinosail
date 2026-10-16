package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
	sharedmetadata "github.com/MikeO7/kinosail/packages/metadata"
)

func (probe *mediaProbe) decorate(ctx context.Context, items []library.Item) []library.Item {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Decorate(ctx, items, probe.enrichment())
}

func libraryDecoration(metadata *metadataStore, probe *mediaProbe, legacy *sharedmetadata.TMDBClient, background bool) catalog.Decoration {
	return catalog.Decoration{
		Apply: metadata.apply, Background: background,
		Enrich: func(ctx context.Context, items []library.Item) []library.Item {
			return legacy.Enrich(ctx, probe.decorate(ctx, items))
		},
		Enriches: func(item library.Item) bool {
			return mediaprobe.Decorates(item) || item.Kind == "video" && item.Show == ""
		},
	}
}
