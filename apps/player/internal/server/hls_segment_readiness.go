package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
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
		job, err := manager.prepareRecipeSegmentFile(segmentContext, item, recipe, name, path)
		if err != nil {
			cancel()
			return false
		}
		ready, err = waitForHLSJobFile(segmentContext, path, job)
		if ready && job != nil {
			err = manager.validateRecipeSegmentPolicy(segmentContext, item, recipe, job.cachePolicy)
			if err != nil {
				slog.WarnContext(request.Context(), "HLS segment preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(request.Context()), "playback_session", requestPlaybackSession(request.Context()), "mode", recipe.mode, "canceled", request.Context().Err() != nil, "error", hlsDiagnostic(err, item.Path))
				cancel()
				return false
			}
			job.observation.emit("media_ready", "")
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

func (manager *hlsManager) validateRecipeSegmentPolicy(ctx context.Context, item library.Item, recipe hlsRecipe, expected string) error {
	resolved, err := playback.ResolveHLSSource(sharedHLSRecipe(recipe), mediaFactsFor(item, manager.probe.facts(ctx, item)), item.Subtitles)
	if err != nil {
		return err
	}
	return manager.validateHLSPolicy(ctx, item, localHLSRecipe(resolved), expected)
}

func (manager *hlsManager) prepareRecipeSegmentFile(ctx context.Context, item library.Item, recipe hlsRecipe, name, path string) (*hlsJob, error) {
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	job, err := manager.prepareSegmentJob(ctx, item, recipe, name)
	if err != nil {
		slog.WarnContext(ctx, "HLS segment preparation failed", "diagnostic", "[PLAYBACK-HLS]", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "mode", recipe.mode, "canceled", ctx.Err() != nil, "error", hlsDiagnostic(err, item.Path))
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	}
	return job, err
}

func waitForHLSJobFile(ctx context.Context, path string, job *hlsJob) (bool, error) {
	var failure error
	directories := []string{filepath.Dir(path)}
	if job == nil {
		ready := playback.WaitHLSChange(ctx, directories, nil, func() error {
			_, failure = os.Stat(path)
			return failure
		})
		return ready, failure
	}
	ready := playback.WaitHLSChange(ctx, directories, job.done, func() error {
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
