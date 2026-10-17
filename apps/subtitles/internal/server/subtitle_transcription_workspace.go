package server

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

const (
	subtitleTranscriptionWorkspacePrefix = "kinosail-transcription-"
	subtitleTranscriptionSweepLimit      = 1024
)

func sweepSubtitleTranscriptionWorkspaces(cacheDir string) {
	if cacheDir == "" {
		return
	}
	directory, err := os.Open(cacheDir)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		slog.Warn("transcription workspace sweep failed", "operation", "subtitle.transcription.sweep", "error_class", "filesystem")
		return
	}
	entries, err := directory.ReadDir(subtitleTranscriptionSweepLimit)
	_ = directory.Close()
	if err != nil && !errors.Is(err, io.EOF) {
		slog.Warn("transcription workspace sweep failed", "operation", "subtitle.transcription.sweep", "error_class", "filesystem")
		return
	}
	removed, failed := 0, 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), subtitleTranscriptionWorkspacePrefix) {
			continue
		}
		if os.RemoveAll(filepath.Join(cacheDir, entry.Name())) != nil {
			failed++
			continue
		}
		removed++
	}
	if failed > 0 {
		slog.Warn("stale transcription workspaces were not removed", "operation", "subtitle.transcription.sweep", "removed", removed, "failed", failed, "error_class", "filesystem")
	} else if removed > 0 {
		slog.Info("stale transcription workspaces removed", "operation", "subtitle.transcription.sweep", "removed", removed)
	}
}
