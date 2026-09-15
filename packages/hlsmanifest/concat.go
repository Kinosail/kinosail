// Package hlsmanifest writes safe HTTP Live Streaming input manifests.
package hlsmanifest

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

// WriteSkipConcat writes an FFmpeg concat manifest without the omitted ranges.
func WriteSkipConcat(directory, source string, duration float64, omitted []playback.Range) (string, error) {
	if strings.ContainsAny(source, "\r\n") {
		return "", errors.New("library content path cannot be safely remuxed")
	}
	var manifest strings.Builder
	manifest.WriteString("ffconcat version 1.0\n")
	start := 0.0
	for _, cut := range omitted {
		writeSpan(&manifest, source, start, cut.Start)
		start = cut.End
	}
	writeSpan(&manifest, source, start, duration)
	path := filepath.Join(directory, "skip.ffconcat")
	//nolint:gosec // The caller supplies a generated rendition below the validated cache root.
	return path, os.WriteFile(path, []byte(manifest.String()), 0o600)
}

func writeSpan(manifest *strings.Builder, source string, start, end float64) {
	if end <= start {
		return
	}
	manifest.WriteString("file '")
	manifest.WriteString(strings.ReplaceAll(source, "'", "'\\''"))
	manifest.WriteString("'\n")
	if start > 0 {
		manifest.WriteString("inpoint ")
		manifest.WriteString(strconv.FormatFloat(start, 'f', 6, 64))
		manifest.WriteByte('\n')
	}
	manifest.WriteString("outpoint ")
	manifest.WriteString(strconv.FormatFloat(end, 'f', 6, 64))
	manifest.WriteByte('\n')
}
