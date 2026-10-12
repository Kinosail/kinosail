package server

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

func (manager *hlsManager) scanCopiedHLSSuffix(ctx context.Context, item library.Item, policy string, requested, duration float64) (*copiedHLSTimeline, error) {
	timeline := &copiedHLSTimeline{Policy: policy, Strategy: "h264-idr-keys-1"}
	var end int64
	origin, originKnown := -1.0, false
	arguments := []string{
		"-v", "error", "-threads", "1", "-read_intervals", copiedHLSTime(requested) + "%",
		"-select_streams", "v:0", "-show_packets", "-show_streams", "-show_format",
		"-show_entries", "packet=pts,pts_time,dts,duration,duration_time,flags:stream=time_base:format=start_time:format_tags=:stream_tags=:stream_disposition=",
		"-of", "compact=p=0", item.Path,
	}
	// Stream time_base arrives after packets. Check decimal packet times while
	// reading, so a demux seek far before the requested point cannot scan a
	// whole long file before the rational end/window validation rejects it.
	err := copiedHLSLines(ctx, manager.probe.executable, arguments, 8<<20, 100_010, func(line string) error {
		fields := copiedHLSFields(line)
		if value, ok := fields["start_time"]; ok {
			start, err := strconv.ParseFloat(value, 64)
			originKnown = err == nil && start == 0
			if !originKnown {
				return errCopiedHLSIndex
			}
			return nil
		}
		if _, ok := fields["time_base"]; !ok {
			if !boundedCopiedHLSSuffixPacket(fields, &origin, requested, duration) {
				return errCopiedHLSIndex
			}
		}
		return timeline.packet(line, &end)
	})
	timeline.End = float64(end) * timeline.TimeBase
	if err != nil || !originKnown || !validCopiedHLSTimeline(timeline) {
		return nil, errCopiedHLSIndex
	}
	if !validCopiedHLSSuffixEnd(timeline, origin, requested, duration) {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}

func boundedCopiedHLSSuffixPacket(fields map[string]string, origin *float64, requested, duration float64) bool {
	pts, ptsErr := strconv.ParseFloat(fields["pts_time"], 64)
	length, lengthErr := strconv.ParseFloat(fields["duration_time"], 64)
	if ptsErr != nil || lengthErr != nil || pts < 0 || !validCopiedHLSEnd(pts+length) || length <= 0 || math.IsNaN(length) {
		return false
	}
	if *origin < 0 {
		if !validCopiedHLSSuffixOrigin(fields, pts, requested, duration) {
			return false
		}
		*origin = pts
	}
	return pts >= *origin && pts+length-*origin <= maximumCopiedHLSSuffix+0.000001
}

func validCopiedHLSSuffixEnd(timeline *copiedHLSTimeline, origin, requested, duration float64) bool {
	first := timeline.point(0)
	return !(first > requested || requested >= timeline.End || math.Abs(first-origin) > 0.000001 ||
		math.Abs(timeline.End-duration) > 0.1 || timeline.End-first > maximumCopiedHLSSuffix || duration-first > maximumCopiedHLSSuffix)
}

func validCopiedHLSSuffixOrigin(fields map[string]string, pts, requested, duration float64) bool {
	return strings.Contains(fields["flags"], "K") && pts <= requested && duration-pts <= maximumCopiedHLSSuffix
}
