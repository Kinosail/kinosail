package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *hlsManager) reportHLSEncode(job *hlsJob, item library.Item, directory string, options transcodeSettings, recipe hlsRecipe, outcome hlsEncodeOutcome, started time.Time) {
	if job.err != nil {
		job.err = newHLSDiagnosticError(job.err, hlsDiagnostic(job.err, item.Path, directory), item.Path, directory)
		slog.Error("HLS transcode failed", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator, "software_fallback", outcome.softwareFallback, "duration_ms", time.Since(started).Milliseconds(), "error", hlsDiagnostic(job.err))
		if !outcome.preserve {
			_ = os.RemoveAll(directory) //nolint:gosec // The key is a validated cache name.
		}
		return
	}
	if !outcome.superseded && !outcome.inactive {
		slog.Info("HLS transcode completed", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "accelerator", options.Accelerator, "software_fallback", outcome.softwareFallback, "duration_ms", time.Since(started).Milliseconds())
	} else if outcome.inactive && outcome.preserve {
		slog.Info("HLS transcode paused after playback became inactive", "request_id", job.requestID, "playback_session", job.playbackSession, "mode", recipe.mode, "duration_ms", time.Since(started).Milliseconds())
	}
}

func (manager *hlsManager) finishHLSEncode(job *hlsJob, key, directory string, startNumber int, preserve bool) {
	if preserve {
		_ = os.RemoveAll(filepath.Join(directory, hlsSeekDirectory(startNumber))) //nolint:gosec // The directory uses a validated segment number.
	}
	job.cancel(nil)
	manager.mu.Lock()
	if manager.jobs[key] == job {
		delete(manager.jobs, key)
	}
	close(job.done)
	manager.mu.Unlock()
}

type hlsEncodeOutcome struct {
	softwareFallback bool
	superseded       bool
	inactive         bool
	preserve         bool
}

func (manager *hlsManager) encode(ctx context.Context, item library.Item, job *hlsJob, key string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) {
	started := time.Now()
	job.observation.queued(recipe.mode)
	directory := filepath.Join(manager.cache, key)
	manager.prepareHLSEncodeDirectory(job, directory, preserve)
	outcome := hlsEncodeOutcome{preserve: preserve}
	if job.err == nil {
		outcome = manager.runHLSEncode(ctx, item, job, directory, options, recipe, startNumber, preserve)
		manager.reportHLSEncode(job, item, directory, options, recipe, outcome, started)
	}
	manager.finishHLSEncode(job, key, directory, startNumber, outcome.preserve)
}

func (manager *hlsManager) prepareHLSEncodeDirectory(job *hlsJob, directory string, preserve bool) {
	if !preserve {
		job.err = os.RemoveAll(directory) //nolint:gosec // The key is a validated cache name.
	}
	if job.err == nil {
		job.err = os.MkdirAll(directory, 0o700) //nolint:gosec // The key is a validated cache name.
	}
}

func (manager *hlsManager) runHLSEncode(ctx context.Context, item library.Item, job *hlsJob, directory string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) hlsEncodeOutcome {
	outcome := hlsEncodeOutcome{preserve: preserve}
	job.err = manager.encodeVariants(ctx, item, directory, options, recipe, startNumber)
	outcome.superseded = errors.Is(context.Cause(ctx), errHLSSeekRestart) || errors.Is(context.Cause(ctx), errHLSIdentityChanged)
	outcome.inactive = errors.Is(context.Cause(ctx), errHLSInactive)
	if outcome.superseded || outcome.inactive {
		job.err = nil
	}
	if outcome.inactive && !outcome.preserve {
		outcome.preserve = preserveInactiveHLS(job, directory, item.Path, options.Cache)
	}
	if manager.retrySoftwareHLSEncode(ctx, item, job, directory, options, recipe, startNumber, outcome.preserve) {
		outcome.softwareFallback = true
	}
	if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil && job.err != nil {
		outcome.preserve = true
	}
	return outcome
}

func preserveInactiveHLS(job *hlsJob, directory, source, cache string) bool {
	preserve := false
	if masterFresh(filepath.Join(directory, "index.m3u8"), source, cache) {
		job.err = writeAtomicFile(filepath.Join(directory, ".seekable"), []byte(cache))
		preserve = job.err == nil
	}
	if !preserve {
		_ = os.RemoveAll(directory) //nolint:gosec // The key is a validated cache name.
	}
	return preserve
}
