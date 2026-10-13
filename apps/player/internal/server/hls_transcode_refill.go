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
	target, end, err := recovery.refillWindow(start)
	if err != nil || !recovery.includesQualities(qualities) {
		return nil, errTranscodeHLSClock
	}
	clock := recovery.clock.Renditions[recovery.rendition]
	rate := recovery.clock.Source.Rate
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

func (recovery *transcodeHLSRecovery) refillWindow(start float64) (int64, float64, error) {
	clock := recovery.clock.Renditions[recovery.rendition]
	rate := recovery.clock.Source.Rate
	target := int64(math.Round(start * float64(rate)))
	cut := clock.Cuts[recovery.number]
	if math.Abs(start-float64(cut.VideoPTS)/float64(clock.VideoBase)) > 0.000001 || math.Abs(start*float64(rate)-float64(target)) > 0.000001 || start < 0 || target >= recovery.clock.Source.Samples {
		return 0, 0, errTranscodeHLSClock
	}
	end := float64(recovery.clock.Source.Samples) / float64(rate)
	if recovery.number+2 < len(clock.Cuts) {
		end = float64(clock.Cuts[recovery.number+2].VideoPTS)/float64(clock.VideoBase) + 0.5
	}
	if end <= start || end-start > 12 {
		return 0, 0, errTranscodeHLSClock
	}
	return target, end, nil
}

func (recovery *transcodeHLSRecovery) includesQualities(qualities []PlaybackQuality) bool {
	clock := recovery.clock.Renditions[recovery.rendition]
	cut := clock.Cuts[recovery.number]
	for _, quality := range qualities {
		other, ok := recovery.clock.Renditions[quality.Label]
		if !ok || other.Rate != clock.Rate || other.VideoBase <= 0 || recovery.number >= len(other.Cuts) || other.Cuts[recovery.number].VideoPTS*clock.VideoBase != cut.VideoPTS*other.VideoBase {
			return false
		}
	}
	return true
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
		bytes, valid := output.renditionSize(media)
		total += bytes
		_ = media.Close()
		if !valid || total > 512<<20 {
			return false
		}
	}
	return true
}

func (output *transcodeHLSOutput) renditionSize(media *os.Root) (int64, bool) {
	file, err := media.Open(".")
	if err != nil {
		return 0, false
	}
	defer file.Close()
	entries, err := file.ReadDir(17)
	if len(entries) > 16 || err != nil && !errors.Is(err, io.EOF) {
		return 0, false
	}
	var total int64
	for _, entry := range entries {
		size, valid := output.fileSize(media, entry.Name())
		if !valid {
			return 0, false
		}
		total += size
	}
	return total, true
}

func (output *transcodeHLSOutput) fileSize(media *os.Root, filename string) (int64, bool) {
	limit := output.fileLimit(filename)
	if limit == 0 {
		return 0, false
	}
	info, err := media.Lstat(filename)
	if os.IsNotExist(err) && strings.HasSuffix(filename, ".tmp") {
		return 0, true
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
		return 0, false
	}
	return info.Size(), true
}

func (output *transcodeHLSOutput) fileLimit(filename string) int64 {
	name := strings.TrimSuffix(filename, ".tmp")
	number, segment := hlsSegmentNumber(name)
	limit := int64(64 << 20)
	if name == "init.mp4" || name == "index.m3u8" {
		limit = 2 << 20
	} else if !segment || number < output.recovery.number || number > output.recovery.number+6 {
		return 0
	}
	return limit
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
