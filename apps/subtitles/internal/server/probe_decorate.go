package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/catalog"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

func (probe *mediaProbe) decorate(ctx context.Context, items []library.Item) []library.Item {
	probe.core.ConfigureCache(probe.cacheDir)
	return probe.core.Decorate(ctx, items, mediaprobe.Enrichment{})
}

func libraryDecoration(metadata *metadataStore, probe *mediaProbe, background bool) catalog.Decoration {
	decoration := catalog.Decoration{Apply: metadata.apply, Enrich: probe.decorate, Enriches: mediaprobe.Decorates, Background: background}
	if background {
		decoration.StartupWait = catalog.ManagedStartupWait
	}
	return decoration
}
