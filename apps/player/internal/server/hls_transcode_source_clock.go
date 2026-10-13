package server

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/mediaprobe"
)

// Completion certification is optional. Only a closed, zero-origin fixed-block
// FLAC clock currently supports reconstructing samples before accurate trimming.
type transcodeSourceClock struct {
	Rate, Base, Quantum, Samples int64
}

type transcodeSourceProjection struct {
	clock               transcodeSourceClock
	low, high, previous int64
	shape               string
	frames              int
}

func (manager *hlsManager) transcodeAudioSourceClock(ctx context.Context, item library.Item, recipe hlsRecipe) (*transcodeSourceClock, error) {
	if recipe.offset != 0 || len(recipe.omitted) != 0 || recipe.dialogueBoost || recipe.normalizeLoudness {
		return nil, errTranscodeHLSClock
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	var projection transcodeSourceProjection
	arguments := []string{"-v", "error", "-threads", "1", "-select_streams", "a:" + strconv.Itoa(recipe.audio), "-show_streams", "-show_entries", "stream=codec_name,sample_rate,time_base,start_pts:stream_tags=:stream_disposition=", "-of", "compact=p=0", item.Path}
	logger := slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx))
	if err := mediaprobe.ProbeLines(ctx, logger, manager.probe.executable, arguments, 8192, 1, projection.header); err != nil {
		return nil, err
	}
	if projection.clock.Rate == 0 {
		return nil, errTranscodeHLSClock
	}
	arguments = []string{"-v", "error", "-threads", "1", "-select_streams", "a:" + strconv.Itoa(recipe.audio), "-show_frames", "-show_entries", "frame=pts,nb_samples,channels,channel_layout,sample_fmt:frame_side_data", "-of", "compact=p=0", item.Path}
	if err := mediaprobe.ProbeLines(ctx, logger, manager.probe.executable, arguments, 192<<20, 2_000_000, projection.frame); err != nil {
		return nil, err
	}
	if projection.frames < 3 {
		return nil, errTranscodeHLSClock
	}
	return &projection.clock, nil
}

func (projection *transcodeSourceProjection) header(line string) error {
	fields := mediaprobe.CompactFields(line)
	rate, base, valid := copiedHLSAudioSourceHeader(fields)
	if projection.clock.Rate != 0 || !valid || fields["codec_name"] != "flac" || base < 1000 {
		return errTranscodeHLSClock
	}
	projection.clock.Rate, projection.clock.Base, projection.high = rate, base, rate-1
	return nil
}

func (projection *transcodeSourceProjection) frame(line string) error {
	result := &projection.clock
	pts, count, format, err := transcodeSourceFrame(line)
	if err != nil || result.Samples > 1<<32-count {
		return errTranscodeHLSClock
	}
	if !projection.acceptFrameShape(pts, count, format) {
		return errTranscodeHLSClock
	}
	if !projection.acceptTimestamp(pts) {
		return errTranscodeHLSClock
	}
	result.Samples += count
	projection.previous = count
	projection.frames++
	return nil
}

func (projection *transcodeSourceProjection) acceptFrameShape(pts, count int64, format string) bool {
	clock := &projection.clock
	if projection.frames > 0 {
		return format == projection.shape && count <= clock.Quantum && projection.previous == clock.Quantum
	}
	uncertainty := 2*((clock.Rate+clock.Base-1)/clock.Base) + 2
	if pts != 0 || 2*uncertainty >= count {
		return false
	}
	clock.Quantum, projection.shape = count, format
	return true
}

func transcodeSourceFrame(line string) (int64, int64, string, error) {
	fields := mediaprobe.CompactFields(line)
	pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
	count, countErr := strconv.ParseInt(fields["nb_samples"], 10, 32)
	format, valid := copiedHLSAudioFrameShape(fields)
	// Unknown frame/side-data fields are not admitted as a trimming contract.
	if ptsErr != nil || countErr != nil || !valid || count <= 0 || count > 65536 || len(fields) != 5 {
		return 0, 0, "", errTranscodeHLSClock
	}
	return pts, count, format, nil
}

func (projection *transcodeSourceProjection) acceptTimestamp(pts int64) bool {
	clock := projection.clock
	whole, remainder := clock.Samples*clock.Base/clock.Rate, clock.Samples*clock.Base%clock.Rate
	if pts < whole || pts > whole+1 {
		return false
	}
	bound := (pts-whole)*clock.Rate - remainder
	projection.low, projection.high = max(projection.low, bound), min(projection.high, bound+clock.Rate-1)
	return projection.low <= projection.high
}
