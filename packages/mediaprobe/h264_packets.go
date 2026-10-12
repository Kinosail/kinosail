package mediaprobe

import (
	"context"
	"math"
	"strconv"
	"strings"
)

func (timeline *h264Timeline) packet(line string, end *int64) error {
	fields := CompactFields(line)
	if value, ok := fields["time_base"]; ok {
		return timeline.setTimeBase(value)
	}
	pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
	duration, durationErr := strconv.ParseInt(fields["duration"], 10, 64)
	dts, dtsErr := strconv.ParseInt(fields["dts"], 10, 64)
	if !validCopiedHLSPacket(pts, duration, ptsErr, durationErr) {
		return errCopiedHLSIndex
	}
	if strings.Contains(fields["flags"], "K") {
		if err := timeline.recordKey(pts, dts, dtsErr); err != nil {
			return err
		}
	}
	if len(timeline.Keys) == 0 || pts < timeline.Keys[len(timeline.Keys)-1].PTS {
		return errCopiedHLSIndex
	}
	*end = max(*end, pts+duration)
	return nil
}

// CompactFields splits bounded ffprobe compact output without interpreting values.
func CompactFields(line string) map[string]string {
	fields := make(map[string]string)
	for _, field := range strings.Split(line, "|") {
		if key, value, ok := strings.Cut(field, "="); ok {
			fields[key] = value
		}
	}
	return fields
}

func validCopiedHLSPacket(pts, duration int64, ptsErr, durationErr error) bool {
	return ptsErr == nil && durationErr == nil && pts >= 0 && pts <= 1<<52 && duration > 0 && duration <= 1<<40
}

func (timeline *h264Timeline) recordKey(pts, dts int64, dtsErr error) error {
	if len(timeline.Keys) >= maximumH264Keys || len(timeline.Keys) > 0 && pts <= timeline.Keys[len(timeline.Keys)-1].PTS {
		return errCopiedHLSIndex
	}
	if dtsErr != nil && len(timeline.Keys) > 0 {
		return errCopiedHLSIndex
	}
	timeline.Keys = append(timeline.Keys, H264Key{PTS: pts, DTS: dts})
	return nil
}

func (timeline *h264Timeline) setTimeBase(value string) error {
	numerator, denominator, found := strings.Cut(value, "/")
	a, aErr := strconv.ParseInt(numerator, 10, 32)
	b, bErr := strconv.ParseInt(denominator, 10, 32)
	if !found || aErr != nil || bErr != nil || a <= 0 || b <= 0 {
		return errCopiedHLSIndex
	}
	timeline.Numerator, timeline.Denominator = a, b
	timeline.TimeBase = float64(a) / float64(b)
	return nil
}

func (probe *Probe) scanH264Packets(ctx context.Context, options H264Scan) (*h264Timeline, error) {
	timeline := &h264Timeline{}
	var end int64
	originKnown := false
	arguments := []string{
		"-v", "error", "-threads", "1", "-select_streams", "v:0", "-show_packets", "-show_streams", "-show_format",
		"-show_entries", "packet=pts,dts,duration,flags:stream=time_base:format=start_time:format_tags=:stream_tags=:stream_disposition=", "-of", "compact=p=0", options.Path,
	}
	err := ProbeLines(ctx, options.Logger, probe.executable, arguments, options.MaximumPacketBytes, options.MaximumPacketLines, func(line string) error {
		if value, ok := CompactFields(line)["start_time"]; ok {
			origin, err := strconv.ParseFloat(value, 64)
			originKnown = err == nil && origin == 0
			if !originKnown {
				return errCopiedHLSIndex
			}
			return nil
		}
		return timeline.packet(line, &end)
	})
	timeline.End = float64(end) * timeline.TimeBase
	if err != nil || !originKnown || !validH264Timeline(timeline) {
		return nil, errCopiedHLSIndex
	}
	if timeline.End <= timeline.point(len(timeline.Keys)-1) || math.Abs(timeline.End-options.Duration) > 0.1 || timeline.End-timeline.point(0) > options.MaximumSpan {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}
