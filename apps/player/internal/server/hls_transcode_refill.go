package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type transcodeHLSOutputKey struct{}

type transcodeHLSOutput struct {
	recovery     *transcodeHLSRecovery
	directory    string
	stage        *os.Root
	warm, target int64
	duration     float64
	qualities    []PlaybackQuality
}

func (manager *hlsManager) refillTranscodeHLS(ctx context.Context, item library.Item, directory string, options transcodeSettings, recipe hlsRecipe, qualities []PlaybackQuality, audioBitrate int64, start float64, recovery *transcodeHLSRecovery) error {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	output, err := newTranscodeHLSOutput(manager.cache, recovery, qualities, start)
	if err != nil {
		return err
	}
	defer output.close()
	ctx = context.WithValue(ctx, transcodeHLSOutputKey{}, output)
	stop, joined := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(joined)
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !output.bounded() {
					cancel()
					return
				}
			}
		}
	}()
	err = manager.encodePresentation(ctx, item, output.directory, options, recipe, qualities, audioBitrate, start, recovery.number)
	close(stop)
	<-joined
	if err != nil || ctx.Err() != nil || !output.bounded() {
		return errTranscodeHLSClock
	}
	return output.publish(manager, ctx, item, directory, options.Cache)
}

func newTranscodeHLSOutput(cache string, recovery *transcodeHLSRecovery, qualities []PlaybackQuality, start float64) (*transcodeHLSOutput, error) {
	clock := recovery.clock.Renditions[recovery.rendition]
	rate := recovery.clock.Source.Rate
	target := int64(math.Round(start * float64(rate)))
	cut := clock.Cuts[recovery.number]
	if math.Abs(start-float64(cut.VideoPTS)/float64(clock.VideoBase)) > 0.000001 || math.Abs(start*float64(rate)-float64(target)) > 0.000001 || start < 0 || target >= recovery.clock.Source.Samples {
		return nil, errTranscodeHLSClock
	}
	end := float64(recovery.clock.Source.Samples) / float64(rate)
	if recovery.number+2 < len(clock.Cuts) {
		end = float64(clock.Cuts[recovery.number+2].VideoPTS)/float64(clock.VideoBase) + 0.5
	}
	if end <= start || end-start > 12 {
		return nil, errTranscodeHLSClock
	}
	for _, quality := range qualities {
		other, ok := recovery.clock.Renditions[quality.Label]
		if !ok || other.Rate != rate || other.VideoBase <= 0 || recovery.number >= len(other.Cuts) || other.Cuts[recovery.number].VideoPTS*clock.VideoBase != cut.VideoPTS*other.VideoBase {
			return nil, errTranscodeHLSClock
		}
	}
	phase := (clock.Cuts[0].AudioPTS%1024 + 1024) % 1024
	limit := max(int64(0), target-3*rate)
	warm := max(int64(0), limit-((limit-phase)%1024+1024)%1024)
	directory, err := os.MkdirTemp(cache, ".transcode-refill-")
	if err != nil {
		return nil, errTranscodeHLSClock
	}
	stage, err := os.OpenRoot(directory)
	if err != nil {
		_ = os.Remove(directory) // Exclusive empty task-owned stage.
		return nil, errTranscodeHLSClock
	}
	return &transcodeHLSOutput{recovery: recovery, directory: directory, stage: stage, warm: warm, target: target, duration: end - start, qualities: qualities}, nil
}

func (output *transcodeHLSOutput) input(arguments []string, path string) []string {
	rate := output.recovery.clock.Source.Rate
	micros := output.warm * 1_000_000 / rate
	seek := fmt.Sprintf("%d.%06d", micros/1_000_000, micros%1_000_000)
	return append(arguments, "-ss", seek, "-noaccurate_seek", "-i", path)
}

func (output *transcodeHLSOutput) audio(arguments []string, track int, label string) []string {
	integer := func(value int64) string { return strconv.FormatInt(value, 10) }
	source := output.recovery.clock.Source
	micros := output.warm * 1_000_000 / source.Rate
	rate, quantum := integer(source.Rate), integer(source.Quantum)
	filter := "asettb=1/" + rate + ",asetpts=round((PTS+" + integer(micros) + "*" + rate + "/1000000)/" + quantum + ")*" + quantum + ",atrim=start_pts=" + integer(output.warm) + ",asetpts=PTS-" + integer(output.target)
	clock := output.recovery.clock.Renditions[label]
	cut := clock.Cuts[output.recovery.number]
	numerator := cut.VideoDTS*source.Rate - output.target*clock.VideoBase
	drop := "noise=amount=0:drop=lt(pts\\," + integer(numerator) + "/" + integer(clock.VideoBase) + ")"
	return append(arguments, "-map", "1:a:"+strconv.Itoa(track), "-af", filter, "-bsf:a", drop)
}

func (output *transcodeHLSOutput) bounded() bool {
	var total int64
	for _, quality := range output.qualities {
		media, err := output.stage.OpenRoot(quality.Label)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false
		}
		file, err := media.Open(".")
		if err != nil {
			_ = media.Close()
			return false
		}
		entries, readErr := file.ReadDir(17)
		_ = file.Close()
		valid := len(entries) <= 16 && (readErr == nil || errors.Is(readErr, io.EOF))
		for _, entry := range entries {
			name := strings.TrimSuffix(entry.Name(), ".tmp")
			number, segment := hlsSegmentNumber(name)
			limit := int64(64 << 20)
			if name == "init.mp4" || name == "index.m3u8" {
				limit = 2 << 20
			} else if !segment || number < output.recovery.number || number > output.recovery.number+6 {
				valid = false
			}
			info, err := media.Lstat(entry.Name())
			if os.IsNotExist(err) && strings.HasSuffix(entry.Name(), ".tmp") {
				continue
			}
			if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
				valid = false
			} else {
				total += info.Size()
			}
		}
		_ = media.Close()
		if !valid || total > 512<<20 {
			return false
		}
	}
	return true
}

func (output *transcodeHLSOutput) close() {
	for _, quality := range output.qualities {
		media, err := output.stage.OpenRoot(quality.Label)
		if err == nil {
			copiedHLSSettleStage(media, filepath.Join(output.directory, quality.Label))
			_ = media.Close()
		}
	}
	copiedHLSSettleStage(output.stage, output.directory)
	_ = output.stage.Close()
}
