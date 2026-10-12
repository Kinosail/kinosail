package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *hlsManager) waitForRecipeSegment(request *http.Request, item library.Item, recipe hlsRecipe, name, key, path string) bool {
	manager.keepHLSAlive(key, request.Context())
	segmentContext, cancel := context.WithTimeout(request.Context(), 30*time.Second)
	if request.Method != http.MethodGet {
		segmentContext = context.WithValue(segmentContext, startupMetadataKey{}, true)
	}
	ready := false
	for attempt := 0; attempt < 2 && !ready; attempt++ {
		var job *hlsJob
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			var err error
			job, err = manager.prepareSegmentJob(segmentContext, item, recipe, name)
			if err != nil {
				slog.WarnContext(request.Context(), "HLS segment preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "mode", recipe.mode, "canceled", request.Context().Err() != nil, "error", hlsDiagnostic(err, item.Path))
				if !errors.Is(err, os.ErrNotExist) {
					cancel()
					return false
				}
			}
		}
		var err error
		ready, err = waitForHLSJobFile(segmentContext, path, job)
		if ready && job != nil {
			manager.mu.Lock()
			err = manager.validateHLSPolicy(segmentContext, item, recipe, job.cachePolicy)
			manager.mu.Unlock()
			if err != nil {
				slog.WarnContext(request.Context(), "HLS segment preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "mode", recipe.mode, "canceled", request.Context().Err() != nil, "error", hlsDiagnostic(err, item.Path))
				cancel()
				return false
			}
		}
		if err != nil {
			break
		}
	}
	if !ready {
		slog.WarnContext(request.Context(), "HLS segment unavailable", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "file", name, "mode", recipe.mode, "canceled", request.Context().Err() != nil)
	}
	cancel()
	return ready
}

// Only a missing file after successful producer completion permits readmission.
// Background preparation keeps using nonblocking prepareSegment directly.
func waitForHLSJobFile(ctx context.Context, path string, job *hlsJob) (bool, error) {
	var failure error
	if job == nil {
		ready := playback.WaitHLSReady(ctx, func() error {
			_, failure = os.Stat(path)
			return failure
		})
		return ready, failure
	}
	ready := playback.WaitHLSReady(ctx, func() error {
		_, failure = os.Stat(path)
		if errors.Is(failure, os.ErrNotExist) {
			select {
			case <-job.done:
				return errHLSSeekRestart
			default:
			}
		}
		return failure
	})
	if ready || ctx.Err() != nil {
		return ready, ctx.Err()
	}
	if !errors.Is(failure, os.ErrNotExist) {
		return false, failure
	}
	if job.err != nil {
		return false, job.err
	}
	//nolint:contextcheck // The joined producer owns its lifecycle independently of this request.
	if !errors.Is(context.Cause(job.lifecycle), errHLSEncodeComplete) {
		return false, context.Canceled
	}
	slog.InfoContext(ctx, "HLS missing segment recovery resumed", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "outcome", "producer-joined")
	return false, nil
}
