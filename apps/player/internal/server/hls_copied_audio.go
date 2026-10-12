package server

import (
	"math"
	"strconv"
)

// Short indexed windows can replay the original AAC encoder history. Dropping
// encoded packets after that replay preserves the uninterrupted packet grid.
func copiedHLSAudioFromOrigin(timeline *copiedHLSTimeline, number int, mode string) bool {
	return timeline != nil && mode == "audio-transcode" && number > 0 &&
		timeline.End-timeline.point(0) <= maximumCopiedHLSSuffix
}

func copiedHLSAudioSeekArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	key, first := timeline.Keys[number], timeline.Keys[0]
	if timeline.Clock == nil || !copiedHLSAudioTicksValid(first, key, timeline.Denominator) {
		return nil, errCopiedHLSIndex
	}
	origin := timeline.point(0)
	input := math.Floor(origin*1_000_000) / 1_000_000
	video := timeline.point(number) - input
	// Subtract ticks before conversion: subtracting two large float timestamps
	// can put an exact packet boundary just above its intended cutoff.
	ticks := float64(key.DTS-first.PTS) * float64(timeline.Numerator)
	audio := ticks / float64(timeline.Denominator)
	// A certificate binds the map, but a decoded field must still be bounded
	// before it becomes a bitstream-filter expression.
	if audio > video || video-audio > 1 || audio < -1 {
		return nil, errCopiedHLSIndex
	}
	// HLS closes audio at the next video's decode time, not its presentation
	// clock. AAC priming can make the mux clock exceed video reorder delay.
	cut := func(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }
	// Compare in AAC sample ticks. Multiplying packet PTS by a floating time
	// base can move exact boundaries. FFmpeg rounds the input seek to samples.
	audioCut := cut(ticks) + "*round(1/tb)/" + strconv.FormatInt(timeline.Denominator, 10) +
		"+round(" + cut(origin-input) + "/tb)"
	return append(arguments, "-copypriorss", "0", "-avoid_negative_ts", "disabled",
		"-output_ts_offset", copiedHLSTime(*timeline.Clock-(origin-input)),
		"-bsf:v", "noise=amount=0:drop=lt(pts*tb\\,"+cut(math.Floor(video*1_000_000)/1_000_000)+")",
		"-bsf:a", "noise=amount=0:drop=lt(pts\\,"+audioCut+")"), nil
}

// FFmpeg time-base denominators are signed 32-bit integers. Together with
// the 30-second window and AAC sample-rate limit, products fit exact doubles.
func copiedHLSAudioTicksValid(first, key copiedHLSKey, denominator int64) bool {
	return first.PTS >= 0 && first.PTS <= 1<<52 && key.DTS >= -(1<<52) && key.DTS <= 1<<52 &&
		denominator > 0 && denominator <= math.MaxInt32
}
