package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodehardware"
)

func (manager *hlsManager) retrySoftwareHLSEncode(ctx context.Context, item library.Item, job *hlsJob, directory string, options transcodeSettings, recipe hlsRecipe) bool {
	if job.err == nil || recipe.mode != "transcode" || options.Accelerator == "none" || ctx.Err() != nil || !transcodehardware.HardwareFailure(hlsDiagnostic(job.err)) {
		return false
	}
	if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil {
		return false
	}
	options = playback.SourceTranscoding(options, mediaFactsFor(item, manager.probe.inspect(ctx, item)), sharedHLSRecipe(recipe))
	colored, err := manager.settings.hardware.ColorSettings(options)
	if err != nil {
		return false
	}
	candidates := manager.settings.hardware.Recovery(colored)
	manager.settings.hardware.RecordProcessingFailure(colored)
	retried := false
	for _, fallback := range candidates {
		if _, err := os.Stat(filepath.Join(directory, "index.m3u8")); err == nil {
			break
		}
		if ctx.Err() != nil {
			break
		}
		if err := resetHLSDirectory(directory); err != nil {
			job.err = err
			break
		}
		retried = true
		job.err = manager.encodeVariants(item, directory, fallback, recipe)
		if job.err == nil || !transcodehardware.HardwareFailure(hlsDiagnostic(job.err)) {
			break
		}
		manager.settings.hardware.RecordFailure(fallback)
	}
	return retried
}

func resetHLSDirectory(directory string) error {
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	return os.MkdirAll(directory, 0o700)
}
