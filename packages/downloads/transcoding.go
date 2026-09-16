package downloads

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// Transcoding adapts Player settings to the download manager's software retry contract.
func Transcoding(current func() transcodepolicy.Settings, softwareEncoder func(string) string) func(bool) transcodepolicy.Settings {
	return func(software bool) transcodepolicy.Settings {
		settings := current()
		if software {
			settings.Accelerator = "none"
			settings.Device = ""
			settings.HardwareDecode = false
			settings.Encoder = softwareEncoder(settings.Codec)
		}
		return settings
	}
}

func (manager *Manager) encodeSelectedContext(ctx context.Context, item library.Item, quality, output string, software bool, selection *TrackSelection) error {
	if !oneOf(quality, "original", "compatible", "1080p", "720p", "480p", "audio") || quality == "audio" && item.Kind == "video" {
		return errors.New("download quality is invalid for this item")
	}
	if quality == "original" {
		return manager.copyOriginal(ctx, item.Path, output)
	}
	options := manager.settings(software)
	if quality == "compatible" {
		options.Codec = "h264"
		options.Encoder = transcodepolicy.DefaultEncoder(options.Accelerator, "h264")
	}
	var plan downloadVideoEncoding
	if item.Kind == "video" {
		var err error
		plan, err = manager.videoEncoding(ctx, item, quality, selection, options)
		if err != nil {
			return err
		}
		options = plan.options
	}
	release, err := manager.reserveEncoding(ctx, item.Kind, plan.copyVideo, options)
	if err != nil {
		return err
	}
	defer release()
	args := []string{"-v", "error"}
	if item.Kind == "video" {
		args = append(args, plan.arguments(item.Path, output)...)
	} else {
		args = append(args, "-i", item.Path, "-vn", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "-y", output)
	}
	//nolint:gosec // Executable is installation configuration. Paths are scanned content and derived cache targets.
	return exec.CommandContext(ctx, manager.ffmpeg, args...).Run()
}

func copyFileContext(ctx context.Context, inputPath, outputPath string) error { //nolint:cyclop // One copy checks source stability and reports copy, sync, and close failures in order.
	if err := ctx.Err(); err != nil {
		return err
	}
	input, err := os.Open(inputPath) //nolint:gosec // The Library scanner supplied this path.
	if err != nil {
		return err
	}
	defer input.Close()
	before, err := input.Stat()
	if err != nil || !before.Mode().IsRegular() {
		return errors.New("download source is invalid")
	}
	output, err := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // Manager derived this cache path.
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, contextReader{ctx, input})
	if copyErr == nil {
		copyErr = output.Sync()
	}
	after, statErr := input.Stat()
	if copyErr == nil && (statErr != nil || !unchangedDownloadSource(before, after)) {
		copyErr = errors.New("download source changed during preparation")
	}
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func (manager *Manager) copyOriginal(ctx context.Context, source, output string) error {
	release, err := manager.reserveContext(ctx)
	if err != nil {
		return err
	}
	defer release()
	return copyFileContext(ctx, source, output)
}

func (manager *Manager) reserveEncoding(ctx context.Context, kind string, copyVideo bool, options transcodepolicy.Settings) (func(), error) {
	if manager.acquireEncoding != nil && kind == "video" && !copyVideo {
		return manager.acquireEncoding(ctx, options)
	}
	return manager.reserveContext(ctx)
}
