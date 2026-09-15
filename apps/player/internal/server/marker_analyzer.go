package server

import (
	"context"

	"github.com/MikeO7/kinosail/packages/library"
	markerlogic "github.com/MikeO7/kinosail/packages/markers"
)

type markerAnalyzer = markerlogic.Analyzer

func markerProbe(probe *mediaProbe) func(context.Context, library.Item) markerlogic.Media {
	return func(ctx context.Context, item library.Item) markerlogic.Media {
		media := probe.inspect(ctx, item)
		return markerlogic.Media{Duration: media.Duration, Markers: media.Markers}
	}
}
