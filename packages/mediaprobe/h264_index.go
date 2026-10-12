package mediaprobe

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

const maximumH264Keys = 4096

// H264Key identifies a certified IDR packet on the source rational clock.
type H264Key struct{ PTS, DTS int64 }

// H264Scan bounds source-only indexing. The caller owns source validation,
// workload admission, and the deadline shared by all three joined probes.
type H264Scan struct {
	Path                        string
	From, Duration, MaximumSpan float64
	MaximumPacketBytes          int64
	MaximumPacketLines          int
	Logger                      *slog.Logger
}

// H264Index contains immutable source cuts. It does not certify generated media,
// cache identity, audio continuity, or permission to publish a playlist.
type H264Index struct{ timeline h264Timeline }

func (index H264Index) TimeBase() (int64, int64) {
	return index.timeline.Numerator, index.timeline.Denominator
}

func (index H264Index) Keys() []H264Key { return append([]H264Key(nil), index.timeline.Keys...) }
func (index H264Index) End() float64    { return index.timeline.End }

// H264Index returns cuts only after packets, IDRs, and codec configuration have
// each reached successful EOF. Unsupported or incomplete proof returns no index.
func (probe *Probe) H264Index(ctx context.Context, ffmpeg string, options H264Scan) (H264Index, error) {
	if ctx.Err() != nil {
		return H264Index{}, ctx.Err()
	}
	if !validH264Scan(options) {
		return H264Index{}, errCopiedHLSIndex
	}
	var timeline *h264Timeline
	var err error
	if options.From > 0 {
		timeline, err = probe.scanH264Suffix(ctx, options)
	} else {
		timeline, err = probe.scanH264Packets(ctx, options)
	}
	if err != nil {
		return H264Index{}, fmt.Errorf("packets: %w", err)
	}
	if err := certifyH264(ctx, options.Logger, ffmpeg, options.Path, timeline); err != nil {
		return H264Index{}, fmt.Errorf("idr: %w", err)
	}
	if err := certifyH264Configuration(ctx, options.Logger, ffmpeg, options.Path, timeline); err != nil {
		return H264Index{}, fmt.Errorf("configuration: %w", err)
	}
	if ctx.Err() != nil {
		return H264Index{}, ctx.Err()
	}
	return H264Index{timeline: *timeline}, nil
}

func validH264Scan(options H264Scan) bool {
	return options.Path != "" && len(options.Path) <= 4096 && !strings.ContainsRune(options.Path, '\x00') &&
		validH264ScanRange(options) &&
		options.MaximumPacketBytes > 0 && options.MaximumPacketBytes <= 128<<20 &&
		options.MaximumPacketLines > 0 && options.MaximumPacketLines <= 2_000_010
}

func validH264ScanRange(options H264Scan) bool {
	return validH264End(options.Duration) && options.From >= 0 && options.From < options.Duration &&
		options.MaximumSpan > 0 && options.MaximumSpan <= 7*24*60*60+0.1
}

type h264Timeline struct {
	Numerator, Denominator int64
	TimeBase               float64
	Keys                   []H264Key
	End                    float64
}

func (timeline *h264Timeline) point(number int) float64 {
	return float64(timeline.Keys[number].PTS) * timeline.TimeBase
}

func validH264Timeline(timeline *h264Timeline) bool {
	if timeline == nil || len(timeline.Keys) == 0 || len(timeline.Keys) > maximumH264Keys ||
		!validH264TimeBase(timeline) || !validH264End(timeline.End) {
		return false
	}
	for number := range timeline.Keys {
		if !validH264Cut(timeline, number) {
			return false
		}
	}
	return true
}

func validH264TimeBase(timeline *h264Timeline) bool {
	return timeline.Numerator > 0 && timeline.Denominator > 0 &&
		timeline.TimeBase == float64(timeline.Numerator)/float64(timeline.Denominator) &&
		timeline.TimeBase > 0 && timeline.TimeBase <= 0.001
}

func validH264Cut(timeline *h264Timeline, number int) bool {
	point, end := timeline.point(number), timeline.End
	if number+1 < len(timeline.Keys) {
		end = timeline.point(number + 1)
	}
	return point >= 0 && point < timeline.End &&
		(number == 0 || point-timeline.point(number-1) >= 0.25) && end-point > 0 && end-point <= 60
}

func validH264End(end float64) bool {
	return end > 0 && end <= 7*24*60*60 && !math.IsNaN(end) && !math.IsInf(end, 0)
}

func h264Time(value float64) string { return strconv.FormatFloat(value, 'f', 6, 64) }
