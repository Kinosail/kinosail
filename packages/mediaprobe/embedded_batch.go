package mediaprobe

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/privatefile"
	"github.com/MikeO7/kinosail/packages/workload"
)

const embeddedBatchLimit = 32

type embeddedOutput struct {
	data []byte
	err  error
}

func embeddedCompanions(result Result, item library.Item, stream int, cacheDir string) []int {
	if embeddedSubtitlePath(cacheDir, item, stream) == "" {
		return nil
	}
	var companions []int
	for _, subtitle := range result.SubtitleFacts {
		index := subtitle.SourceIndex
		if len(companions) == embeddedBatchLimit-1 {
			break
		}
		if !subtitle.Text || index == stream || index < 0 || index > 65535 || slices.Contains(companions, index) {
			continue
		}
		if !privatefile.Exists(embeddedSubtitlePath(cacheDir, item, index), embeddedSubtitleLimit) {
			companions = append(companions, index)
		}
	}
	return companions
}

func extractEmbeddedTargets(ctx context.Context, item library.Item, targets []embeddedTarget, options EmbeddedOptions) {
	if len(targets) > 1 {
		started := time.Now()
		outputs, err := extractEmbeddedBatch(ctx, options.FFmpeg, item.Path, targets)
		if err == nil {
			for position, target := range targets {
				target.call.data, target.call.err = outputs[position].data, outputs[position].err
			}
			return
		}
		if ctx.Err() != nil {
			for _, target := range targets {
				target.call.err = ctx.Err()
			}
			return
		}
		slog.WarnContext(ctx, "combined embedded subtitle extraction failed", "operation", "subtitles.extract", "streams", len(targets), "duration_ms", time.Since(started).Milliseconds(), "error", playback.HLSDiagnostic(err, item.Path, options.CacheDir))
		for _, target := range targets[1:] {
			target.call.err = errEmbeddedRetry
		}
	}
	targets[0].call.data, targets[0].call.err = extractEmbeddedSubtitle(ctx, options.FFmpeg, item.Path, targets[0].stream)
}

func extractEmbeddedBatch(ctx context.Context, ffmpeg, media string, targets []embeddedTarget) ([]embeddedOutput, error) {
	if !validFFmpegExecutable(ffmpeg) {
		return nil, errInvalidFFmpeg
	}
	directory := filepath.Dir(targets[0].path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, err
	}
	workspace, err := os.MkdirTemp(directory, ".extract-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(workspace) }()
	arguments := []string{"-v", "error", "-i", media}
	for _, target := range targets {
		arguments = append(arguments, "-map", "0:"+strconv.Itoa(target.stream), "-f", "webvtt", "-fs", strconv.Itoa(embeddedSubtitleLimit+1), "file:"+embeddedWorkspaceFile(workspace, target.stream))
	}
	//nolint:gosec // The executable is installation config. The media path is scanned content. Stream indexes are probe-validated.
	command := exec.CommandContext(ctx, ffmpeg, arguments...)
	err = workload.RunContext(ctx, command)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	outputs := make([]embeddedOutput, len(targets))
	for position, target := range targets {
		outputs[position].data, outputs[position].err = readEmbeddedOutput(embeddedWorkspaceFile(workspace, target.stream))
	}
	return outputs, nil
}

func embeddedWorkspaceFile(workspace string, stream int) string {
	return filepath.Join(workspace, strconv.Itoa(stream)+".vtt")
}

func readEmbeddedOutput(path string) ([]byte, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, errEmbeddedSubtitleOutput
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, embeddedSubtitleLimit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || len(data) > embeddedSubtitleLimit {
		return nil, errEmbeddedSubtitleOutput
	}
	return data, nil
}
