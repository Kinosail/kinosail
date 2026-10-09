package server

import (
	"fmt"
	"math"
	"math/big"
	"strconv"
)

func copiedHLSProducerArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	if timeline == nil {
		return arguments, nil
	}
	if number < 0 || number >= len(timeline.Keys) || number > 0 && timeline.Clock == nil {
		return nil, errCopiedHLSIndex
	}
	if timeline.AudioOrigin == nil {
		return copiedHLSLegacyProducerArguments(arguments, timeline, number)
	}
	if !validCopiedAACOrigin(timeline) {
		return nil, errCopiedHLSIndex
	}
	if number == 0 {
		return append(arguments, "-copypriorss:v", "0"), nil
	}
	return copiedAACRefillArguments(arguments, timeline, number)
}

func copiedAACRefillArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	if *timeline.Clock != 0 || !validCopiedHLSAudioDTS(timeline, number) {
		return nil, errCopiedHLSIndex
	}
	seek, err := copiedAACKeyMicros(timeline, timeline.Keys[number].PTS)
	if err != nil {
		return nil, err
	}
	mux := seek - timeline.AudioOrigin.InitialSeekMicros
	shift, err := copiedHLSAudioShift(timeline.AudioOrigin.Physical, seek, mux)
	clip, clipErr := copiedAACKeyTicks(timeline, timeline.Keys[number].DTS)
	if err != nil || clipErr != nil {
		return nil, errCopiedHLSIndex
	}
	seekTicks, _ := copiedAACRescale(seek)
	filter := fmt.Sprintf("noise=amount=0:drop=lt(pts+%d\\,%d),setts=pts=PTS+%d:dts=DTS+%d", seekTicks, clip, shift, shift)
	return append(arguments, "-copypriorss:v", "0", "-output_ts_offset", copiedAACMicrosText(mux), "-bsf:a", filter), nil
}

// Unqualified streams retain exact-main argument behavior; the R15 helper remains diagnostic.
func copiedHLSLegacyProducerArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	arguments = append(arguments, "-copypriorss", "0")
	if number == 0 {
		return arguments, nil
	}
	start := timeline.point(number)
	floor := math.Floor(start*1_000_000) / 1_000_000
	offset := start - timeline.point(0) + *timeline.Clock - (start - floor)
	clock := copiedHLSTime(*timeline.Clock)
	return append(arguments, "-output_ts_offset", copiedHLSTime(offset), "-bsf:a", "setts=pts=PTS-"+clock+"/TB:dts=DTS-"+clock+"/TB"), nil
}

func copiedAACMicrosText(value int64) string {
	return strconv.FormatInt(value/1_000_000, 10) + "." + fmt.Sprintf("%06d", value%1_000_000)
}

func copiedAACKeyMicros(timeline *copiedHLSTimeline, pts int64) (int64, error) {
	return copiedAACKeyRescale(timeline, pts, 1_000_000, false)
}

func copiedAACKeyTicks(timeline *copiedHLSTimeline, dts int64) (int64, error) {
	return copiedAACKeyRescale(timeline, dts, 48000, true)
}

func copiedAACKeyRescale(timeline *copiedHLSTimeline, ticks, scale int64, ceil bool) (int64, error) {
	if !validCopiedAACRescaleInput(timeline, ticks) {
		return 0, errCopiedHLSIndex
	}
	numerator := new(big.Int).Mul(big.NewInt(ticks), big.NewInt(timeline.Numerator))
	numerator.Mul(numerator, big.NewInt(scale))
	value, remainder := new(big.Int), new(big.Int)
	value.QuoRem(numerator, big.NewInt(timeline.Denominator), remainder)
	if ceil && remainder.Sign() > 0 {
		value.Add(value, big.NewInt(1))
	}
	if !value.IsInt64() {
		return 0, errCopiedHLSIndex
	}
	result := value.Int64()
	limit := maximumCopiedAACMicros
	if scale == 48000 {
		limit = maximumCopiedAACTicks
	}
	if result < -scale || result > limit {
		return 0, errCopiedHLSIndex
	}
	return result, nil
}

func validCopiedAACRescaleInput(timeline *copiedHLSTimeline, ticks int64) bool {
	return validCopiedHLSTimeBase(timeline) && ticks >= -(1<<52) && ticks <= 1<<52 &&
		timeline.Numerator <= 1<<31 && timeline.Denominator <= 1<<31
}

func copiedAACSegmentArguments(arguments []string, timeline *copiedHLSTimeline, number int) []string {
	arguments = indexedCopiedHLSSegmentArguments(arguments, timeline)
	if timeline == nil || timeline.AudioOrigin == nil {
		return arguments
	}
	flags := "movflags=+skip_sidx:avoid_negative_ts=disabled:use_editlist=1"
	if number > 0 {
		flags = "movflags=+frag_discont+skip_sidx:avoid_negative_ts=disabled:use_editlist=1"
	}
	for index, option := range arguments {
		if option == "-hls_segment_options" && index+1 < len(arguments) {
			arguments[index+1] = flags
		}
	}
	return arguments
}
