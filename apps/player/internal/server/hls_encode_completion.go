package server

import (
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
