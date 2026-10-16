package markers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
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

func (analyzer *Analyzer) slot(ctx context.Context, work func() error) error {
	if analyzer.acquire == nil {
		return work()
	}
	release, err := analyzer.acquire(ctx)
	if err != nil {
		return errors.Join(errAnalysisCapacity, err)
	}
	defer release()
	return work()
}

func (analyzer *Analyzer) fingerprint(ctx context.Context, item library.Item, window string, start, length float64) ([]uint32, error) {
	if !finite(start) || !finite(length) || start < 0 || length <= 0 || length > 900 {
		return nil, errors.New("audio fingerprint window is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	revision := mediaRevision(item)
	path := filepath.Join(analyzer.cache, item.ID+"-"+window+".json")
	var cached fingerprintFile
	if analyzer.cache != "" && readCache(path, &cached) && cached.Revision == revision {
		return cached.Points, nil
	}
	if analyzer.extract == nil && analyzer.cache == "" {
		return nil, errors.New("marker cache is not configured")
	}
	var points []uint32
	err := analyzer.slot(ctx, func() (err error) {
		if analyzer.extract != nil {
			points, err = analyzer.extract(ctx, item, window, start, length)
			return err
		}
		points, err = analyzer.audioFingerprint(ctx, item, start, length)
		return err
	})
	if err != nil {
		return nil, err
	}
	if analyzer.cache != "" {
		_ = saveJSON(path, fingerprintFile{revision, points})
	}
	return points, nil
}

func (analyzer *Analyzer) audioFingerprint(ctx context.Context, item library.Item, start, length float64) ([]uint32, error) {
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
		command := analysisCommand(ctx, analyzer.ffmpeg, analyzer.threads, "-ss", seconds(start), "-t", seconds(length), "-i", item.Path, "-map", "0:a:0", "-ac", "1", "-ar", "11025", "-y", input)
		if output, err := workload.BackgroundCombinedOutput(command); err != nil {
			return nil, errors.New(strings.TrimSpace(string(output)))
		}
	}
	command := exec.CommandContext(ctx, analyzer.tool, "-raw", "-json", "-length", seconds(length), input) //nolint:gosec // Executable is installation config; input is scanned media or a private temporary file.
	data, err := workload.BackgroundOutput(command)
	if err != nil {
		return nil, err
	}
	var output struct {
		Fingerprint []uint32 `json:"fingerprint"`
	}
	if json.Unmarshal(data, &output) != nil || len(output.Fingerprint) == 0 {
		return nil, errors.New("audio fingerprint is unavailable")
	}
	return output.Fingerprint, nil
}

func (analyzer *Analyzer) visualFingerprint(ctx context.Context, item library.Item, window string, start, length float64) ([]uint64, error) {
	if !finite(start) || !finite(length) || start < 0 || length <= 0 || length > 900 {
		return nil, errors.New("visual fingerprint window is invalid")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	revision := mediaRevision(item)
	path := filepath.Join(analyzer.cache, item.ID+"-"+window+"-visual.json")
	var cached visualFingerprintFile
	if analyzer.cache != "" && readCache(path, &cached) && cached.Revision == revision && cached.Version == DetectorVersion {
		return cached.Points, nil
	}
	if analyzer.extractVisual == nil && analyzer.cache == "" {
		return nil, errors.New("marker cache is not configured")
	}
	var points []uint64
	err := analyzer.slot(ctx, func() (err error) {
		if analyzer.extractVisual != nil {
			points, err = analyzer.extractVisual(ctx, item, window, start, length)
			return err
		}
		points, err = analyzer.visualPoints(ctx, item, start, length)
		return err
	})
	if err != nil {
		return nil, err
	}
	if analyzer.cache != "" {
		_ = saveJSON(path, visualFingerprintFile{Revision: revision, Version: DetectorVersion, Points: points})
	}
	return points, nil
}

func (analyzer *Analyzer) visualPoints(ctx context.Context, item library.Item, start, length float64) ([]uint64, error) {
	command := analysisCommand(ctx, analyzer.ffmpeg, analyzer.threads, "-ss", seconds(start), "-i", item.Path, "-t", seconds(length), "-map", "0:v:0", "-vf", "fps=1,scale=9:8:force_original_aspect_ratio=decrease,pad=9:8:(ow-iw)/2:(oh-ih)/2,format=gray", "-an", "-f", "rawvideo", "-pix_fmt", "gray", "-")
	data, err := workload.BackgroundOutput(command)
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
	return points, nil
}

func readCache(path string, target any) bool {
	data, err := os.ReadFile(path) // #nosec G304 -- fixed cache file beneath the installation cache directory.
	return err == nil && json.Unmarshal(data, target) == nil
}

func analysisCommand(ctx context.Context, ffmpeg string, threads int, arguments ...string) *exec.Cmd {
	count := strconv.Itoa(max(1, threads))
	return exec.CommandContext(ctx, ffmpeg, append([]string{"-hide_banner", "-loglevel", "error", "-threads", count, "-filter_threads", count}, arguments...)...) //nolint:gosec // Executable is installation config and media paths are scanned Library Content.
}
