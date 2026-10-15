package server

import (
	"math"
	"strconv"
)

const maximumCopiedHLSAudioReplay = 30.0

func copiedHLSAudioFromOrigin(timeline *copiedHLSTimeline, number int, mode string) bool {
	return timeline != nil && mode == "audio-transcode" && number > 0 &&
		timeline.End-timeline.point(0) <= maximumCopiedHLSAudioReplay
}

func copiedHLSAudioSeekArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	key, first := timeline.Keys[number], timeline.Keys[0]
	if timeline.Clock == nil || !copiedHLSAudioTicksValid(first, key, timeline.Denominator) {
		return nil, errCopiedHLSIndex
	}
	origin := timeline.point(0)
	input := math.Floor(origin*1_000_000) / 1_000_000
	video := timeline.point(number) - input
	ticks := float64(key.DTS-first.PTS) * float64(timeline.Numerator)
	audio := ticks / float64(timeline.Denominator)
	if audio > video || video-audio > 1 || audio < -1 {
		return nil, errCopiedHLSIndex
	}
	cut := func(value float64) string { return strconv.FormatFloat(value, 'g', -1, 64) }
	videoCut := strconv.FormatInt(key.PTS-first.PTS, 10) + "+round(" + cut(origin-input) + "/tb)"
	audioCut := cut(ticks) + "*round(1/tb)/" + strconv.FormatInt(timeline.Denominator, 10) +
		"+round(" + cut(origin-input) + "/tb)"
	return append(arguments, "-copypriorss", "0", "-avoid_negative_ts", "disabled",
		"-output_ts_offset", copiedHLSTime(*timeline.Clock-(origin-input)),
		"-bsf:v", "noise=amount=0:drop=lt(pts\\,"+videoCut+")",
		"-bsf:a", "noise=amount=0:drop=lt(pts\\,"+audioCut+")"), nil
}

func copiedHLSAudioTicksValid(first, key copiedHLSKey, denominator int64) bool {
	return first.PTS >= 0 && first.PTS <= 1<<52 && key.DTS >= -(1<<52) && key.DTS <= 1<<52 &&
		denominator > 0 && denominator <= math.MaxInt32
}
