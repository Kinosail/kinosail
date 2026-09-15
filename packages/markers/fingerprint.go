package markers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type fingerprintFile struct {
	Revision string   `json:"revision"`
	Points   []uint32 `json:"points"`
}

type visualFingerprintFile struct {
	Revision string   `json:"revision"`
	Version  int      `json:"version"`
	Points   []uint64 `json:"points"`
}

func (analyzer *Analyzer) fingerprint(ctx context.Context, item library.Item, window string, start, length float64) ([]uint32, error) { //nolint:cyclop,gocognit // Cache, extraction, and tool failures are handled at one fingerprint boundary.
	if !finite(start) || !finite(length) || start < 0 || length <= 0 || length > 900 {
		return nil, errors.New("audio fingerprint window is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if analyzer.extract != nil {
		return analyzer.extract(ctx, item, window, start, length)
	}
	if analyzer.cache == "" {
		return nil, errors.New("marker cache is not configured")
	}
	revision := mediaRevision(item)
	path := filepath.Join(analyzer.cache, item.ID+"-"+window+".json")
	var cached fingerprintFile
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &cached) == nil && cached.Revision == revision {
		return cached.Points, nil
	}
	input := item.Path
	if start > 0 {
		if err := os.MkdirAll(analyzer.cache, 0o700); err != nil {
			return nil, err
		}
		temporary, err := os.CreateTemp(analyzer.cache, ".audio-*.wav")
		if err != nil {
			return nil, err
		}
		input = temporary.Name()
		_ = temporary.Close()
		defer os.Remove(input)
		command := analysisCommand(ctx, analyzer.ffmpeg, "-ss", seconds(start), "-t", seconds(length), "-i", item.Path, "-map", "0:a:0", "-ac", "1", "-ar", "11025", "-y", input)
		if output, err := command.CombinedOutput(); err != nil {
			return nil, errors.New(strings.TrimSpace(string(output)))
		}
	}
	command := exec.CommandContext(ctx, analyzer.tool, "-raw", "-json", "-length", seconds(length), input) //nolint:gosec // Executable is installation config; input is scanned media or a private temporary file.
	data, err := command.Output()
	if err != nil {
		return nil, err
	}
	var output struct {
		Fingerprint []uint32 `json:"fingerprint"`
	}
	if json.Unmarshal(data, &output) != nil || len(output.Fingerprint) == 0 {
		return nil, errors.New("audio fingerprint is unavailable")
	}
	_ = saveJSON(path, fingerprintFile{revision, output.Fingerprint})
	return output.Fingerprint, nil
}

func (analyzer *Analyzer) visualFingerprint(ctx context.Context, item library.Item, window string, start, length float64) ([]uint64, error) { //nolint:cyclop,gocognit // Cache, extraction, and hashing form one fingerprint boundary.
	if !finite(start) || !finite(length) || start < 0 || length <= 0 || length > 900 {
		return nil, errors.New("visual fingerprint window is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if analyzer.extractVisual != nil {
		return analyzer.extractVisual(ctx, item, window, start, length)
	}
	if analyzer.cache == "" {
		return nil, errors.New("marker cache is not configured")
	}
	revision := mediaRevision(item)
	path := filepath.Join(analyzer.cache, item.ID+"-"+window+"-visual.json")
	var cached visualFingerprintFile
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &cached) == nil && cached.Revision == revision && cached.Version == DetectorVersion {
		return cached.Points, nil
	}
	command := analysisCommand(ctx, analyzer.ffmpeg, "-ss", seconds(start), "-i", item.Path, "-t", seconds(length), "-map", "0:v:0", "-vf", "fps=1,scale=9:8:force_original_aspect_ratio=decrease,pad=9:8:(ow-iw)/2:(oh-ih)/2,format=gray", "-an", "-f", "rawvideo", "-pix_fmt", "gray", "-")
	data, err := command.Output()
	if err != nil || len(data) == 0 || len(data)%72 != 0 {
		return nil, errors.New("visual fingerprint is unavailable")
	}
	points := make([]uint64, len(data)/72)
	for frame := range points {
		pixels := data[frame*72 : (frame+1)*72]
		for row := range 8 {
			for column := range 8 {
				points[frame] <<= 1
				if pixels[row*9+column] > pixels[row*9+column+1] {
					points[frame] |= 1
				}
			}
		}
	}
	_ = saveJSON(path, visualFingerprintFile{Revision: revision, Version: DetectorVersion, Points: points})
	return points, nil
}

func analysisCommand(ctx context.Context, ffmpeg string, arguments ...string) *exec.Cmd {
	return exec.CommandContext(ctx, ffmpeg, append([]string{"-hide_banner", "-loglevel", "error", "-threads", "1", "-filter_threads", "1"}, arguments...)...) //nolint:gosec // Executable is installation config and media paths are scanned Library Content.
}
