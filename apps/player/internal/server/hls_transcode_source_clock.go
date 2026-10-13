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

func (manager *hlsManager) transcodeAudioSourceClock(ctx context.Context, item library.Item, recipe hlsRecipe) (*transcodeSourceClock, error) {
	if recipe.offset != 0 || len(recipe.omitted) != 0 || recipe.dialogueBoost || recipe.normalizeLoudness {
		return nil, errTranscodeHLSClock
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	var result transcodeSourceClock
	var denominator, low, high, previous int64
	var shape string
	frames := 0
	header := func(line string) error {
		fields := mediaprobe.CompactFields(line)
		rate, base, valid := copiedHLSAudioSourceHeader(fields)
		if result.Rate != 0 || !valid || fields["codec_name"] != "flac" || base < 1000 {
			return errTranscodeHLSClock
		}
		result.Rate, result.Base, denominator, high = rate, base, base, rate-1
		return nil
	}
	arguments := []string{"-v", "error", "-threads", "1", "-select_streams", "a:" + strconv.Itoa(recipe.audio), "-show_streams", "-show_entries", "stream=codec_name,sample_rate,time_base,start_pts:stream_tags=:stream_disposition=", "-of", "compact=p=0", item.Path}
	logger := slog.With("request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx))
	if err := mediaprobe.ProbeLines(ctx, logger, manager.probe.executable, arguments, 8192, 1, header); err != nil {
		return nil, err
	}
	if result.Rate == 0 {
		return nil, errTranscodeHLSClock
	}
	frame := func(line string) error {
		fields := mediaprobe.CompactFields(line)
		pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
		count, countErr := strconv.ParseInt(fields["nb_samples"], 10, 32)
		format, valid := copiedHLSAudioFrameShape(fields)
		if ptsErr != nil || countErr != nil || !valid || count <= 0 || count > 65536 || result.Samples > 1<<32-count {
			return errTranscodeHLSClock
		}
		// Unknown frame/side-data fields are not admitted as a trimming contract.
		if len(fields) != 5 {
			return errTranscodeHLSClock
		}
		if frames == 0 {
			uncertainty := 2*((result.Rate+denominator-1)/denominator) + 2
			if pts != 0 || 2*uncertainty >= count {
				return errTranscodeHLSClock
			}
			result.Quantum, shape = count, format
		}
		if format != shape || count > result.Quantum || frames > 0 && previous != result.Quantum {
			return errTranscodeHLSClock
		}
		whole, remainder := result.Samples*denominator/result.Rate, result.Samples*denominator%result.Rate
		if pts < whole || pts > whole+1 {
			return errTranscodeHLSClock
		}
		bound := (pts-whole)*result.Rate - remainder
		low, high = max(low, bound), min(high, bound+result.Rate-1)
		if low > high {
			return errTranscodeHLSClock
		}
		result.Samples += count
		previous = count
		frames++
		return nil
	}
	arguments = []string{"-v", "error", "-threads", "1", "-select_streams", "a:" + strconv.Itoa(recipe.audio), "-show_frames", "-show_entries", "frame=pts,nb_samples,channels,channel_layout,sample_fmt:frame_side_data", "-of", "compact=p=0", item.Path}
	if err := mediaprobe.ProbeLines(ctx, logger, manager.probe.executable, arguments, 192<<20, 2_000_000, frame); err != nil {
		return nil, err
	}
	if frames < 3 {
		return nil, errTranscodeHLSClock
	}
	return &result, nil
}
