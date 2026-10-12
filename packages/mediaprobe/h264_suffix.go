package mediaprobe

import (
	"context"
	"math"
	"strconv"
	"strings"
)

func (probe *Probe) scanH264Suffix(ctx context.Context, options H264Scan) (*h264Timeline, error) {
	requested, duration, maximumSpan := options.From, options.Duration, options.MaximumSpan
	timeline := &h264Timeline{}
	var end int64
	origin, originKnown := -1.0, false
	arguments := []string{
		"-v", "error", "-threads", "1", "-read_intervals", h264Time(requested) + "%",
		"-select_streams", "v:0", "-show_packets", "-show_streams", "-show_format",
		"-show_entries", "packet=pts,pts_time,dts,duration,duration_time,flags:stream=time_base:format=start_time:format_tags=:stream_tags=:stream_disposition=",
		"-of", "compact=p=0", options.Path,
	}
	// Stream time_base arrives after packets. Check decimal packet times while
	// reading, so a demux seek far before the requested point cannot scan a
	// whole long file before the rational end/window validation rejects it.
	err := ProbeLines(ctx, options.Logger, probe.executable, arguments, options.MaximumPacketBytes, options.MaximumPacketLines, func(line string) error {
		fields := CompactFields(line)
		if value, ok := fields["start_time"]; ok {
			start, err := strconv.ParseFloat(value, 64)
			originKnown = err == nil && start == 0
			if !originKnown {
				return errCopiedHLSIndex
			}
			return nil
		}
		if _, ok := fields["time_base"]; !ok {
			if !boundedCopiedHLSSuffixPacket(fields, &origin, requested, duration, maximumSpan) {
				return errCopiedHLSIndex
			}
		}
		return timeline.packet(line, &end)
	})
	timeline.End = float64(end) * timeline.TimeBase
	if err != nil {
		return nil, err
	}
	if !originKnown || !validH264Timeline(timeline) {
		return nil, errCopiedHLSIndex
	}
	if !validCopiedHLSSuffixEnd(timeline, origin, requested, duration, maximumSpan) {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}

func boundedCopiedHLSSuffixPacket(fields map[string]string, origin *float64, requested, duration, maximumSpan float64) bool {
	pts, ptsErr := strconv.ParseFloat(fields["pts_time"], 64)
	length, lengthErr := strconv.ParseFloat(fields["duration_time"], 64)
	if ptsErr != nil || lengthErr != nil || pts < 0 || !validH264End(pts+length) || length <= 0 || math.IsNaN(length) {
		return false
	}
	if *origin < 0 {
		if !validCopiedHLSSuffixOrigin(fields, pts, requested, duration, maximumSpan) {
			return false
		}
		*origin = pts
	}
	return pts >= *origin && pts+length-*origin <= maximumSpan+0.000001
}

func validCopiedHLSSuffixEnd(timeline *h264Timeline, origin, requested, duration, maximumSpan float64) bool {
	first := timeline.point(0)
	return !(first > requested || requested >= timeline.End || math.Abs(first-origin) > 0.000001 ||
		math.Abs(timeline.End-duration) > 0.1 || timeline.End-first > maximumSpan || duration-first > maximumSpan)
}

func validCopiedHLSSuffixOrigin(fields map[string]string, pts, requested, duration, maximumSpan float64) bool {
	return strings.Contains(fields["flags"], "K") && pts <= requested && duration-pts <= maximumSpan
}
