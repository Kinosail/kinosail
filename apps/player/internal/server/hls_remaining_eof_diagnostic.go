package server

import (
	"context"
	"log/slog"

	"github.com/MikeO7/kinosail/packages/library"
)

func remainingColdAACRejected(ctx context.Context, recipe hlsRecipe, item library.Item, err error) {
	slog.WarnContext(ctx, "HLS audio timeline unavailable", "diagnostic", "[PLAYBACK-HLS]",
		"request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx),
		"mode", recipe.mode, "error", hlsDiagnostic(err, item.Path))
}
