package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
)

func (manager *hlsManager) retrySoftwareHLSEncode(ctx context.Context, item library.Item, job *hlsJob, directory string, options transcodeSettings, recipe hlsRecipe, startNumber int, preserve bool) bool { //nolint:cyclop // Recovery must preserve published segments and stop on cancellation, filesystem errors, or a non-hardware failure.
	if job.err == nil || recipe.mode != "transcode" || options.Accelerator == "none" || ctx.Err() != nil || !transcodehardware.HardwareFailure(hlsDiagnostic(job.err)) {
		return false
	}
	if preserve {
		return false
	}
	if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil {
		return false
	}
	options = playback.SourceTranscoding(options, mediaFactsFor(item, manager.probe.facts(ctx, item)), sharedHLSRecipe(recipe))
	options, colorErr := manager.settings.hardware.ColorSettings(options)
	if colorErr != nil {
		return false
	}
	candidates := manager.settings.hardware.Recovery(options)
	manager.recordHLSHardwareFailure(options)
	retried := false
	for _, fallback := range candidates {
		fallback.Cache = options.Cache
		if ctx.Err() != nil {
			break
		}
		if err := resetHLSDirectory(directory); err != nil {
			job.err = err
			break
		}
		retried = true
		job.err = manager.encodeVariants(ctx, item, directory, fallback, recipe, startNumber)
		if job.err == nil || !transcodehardware.HardwareFailure(hlsDiagnostic(job.err)) {
			break
		}
		manager.settings.hardware.RecordFailure(fallback)
		if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil {
			break
		}
	}
	return retried
}

func resetHLSDirectory(directory string) error {
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	return os.MkdirAll(directory, 0o700)
}

func (manager *hlsManager) recordHLSHardwareFailure(options transcodeSettings) {
	manager.settings.hardware.RecordProcessingFailure(options)
}
