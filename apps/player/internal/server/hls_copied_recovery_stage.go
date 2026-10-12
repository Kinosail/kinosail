package server

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
)

func (output *copiedHLSVariantOutput) boundedStage() bool {
	file, err := output.stage.Open(".")
	if err != nil {
		return false
	}
	defer file.Close()
	entries, err := file.ReadDir(9)
	if err != nil && !errors.Is(err, io.EOF) || len(entries) > 8 {
		return false
	}
	total := int64(0)
	for _, entry := range entries {
		size, valid := output.stagedFileSize(entry.Name())
		if !valid {
			return false
		}
		total += size
	}
	return total <= 128<<20
}

func (output *copiedHLSVariantOutput) stagedFileSize(name string) (int64, bool) {
	limit := copiedHLSStagedFileLimit(name)
	if limit == 0 {
		return 0, false
	}
	info, err := output.stage.Lstat(name)
	// FFmpeg can rename a recognized temporary entry after ReadDir sees it.
	if os.IsNotExist(err) && strings.HasSuffix(name, ".tmp") {
		slog.DebugContext(output.observation, "HLS copied temporary output replaced", "request_id", requestActivityID(output.observation), "playback_session", requestPlaybackSession(output.observation), "outcome", "stage-scan-continues")
		return 0, true
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return 0, false
	}
	return info.Size(), true
}

func copiedHLSStagedFileLimit(name string) int64 {
	name = strings.TrimSuffix(name, ".tmp")
	switch name {
	case "index.m3u8":
		return maximumCopiedHLSTimelineBytes
	case "init.mp4":
		return 2 << 20
	}
	if number, valid := hlsSegmentNumber(name); valid && number <= 2 {
		return 64 << 20
	}
	return 0
}
