package mediaprobe

import (
	"context"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

// A key flag alone may be a recovery point. Filtered IDR packets must match the
// complete demux key map in order; uncertain boundaries cannot be prepared.
func certifyH264(ctx context.Context, logger *slog.Logger, ffmpeg, path string, timeline *h264Timeline) error {
	proof := copiedHLSIDRProof{timeline: timeline}
	arguments := copiedHLSCertificationArguments(path, timeline, "filter_units=pass_types=5")
	err := ProbeLines(ctx, logger, ffmpeg, arguments, 1<<20, maximumH264Keys+32, proof.line)
	if err != nil {
		return err
	}
	if proof.count != len(timeline.Keys) {
		return errCopiedHLSIndex
	}
	return nil
}

type copiedHLSIDRProof struct {
	timeline *h264Timeline
	base     float64
	count    int
}

func (proof *copiedHLSIDRProof) line(line string) error {
	if strings.HasPrefix(line, "#tb 0: ") {
		fields := CompactFields("time_base=" + strings.TrimPrefix(line, "#tb 0: "))
		probe := h264Timeline{}
		end := int64(0)
		if err := probe.packet("time_base="+fields["time_base"], &end); err != nil {
			return err
		}
		proof.base = probe.TimeBase
		return nil
	}
	if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
		return nil
	}
	return proof.packet(line)
}

func (proof *copiedHLSIDRProof) packet(line string) error {
	fields := strings.Split(line, ",")
	if len(fields) != 6 || proof.base <= 0 || proof.count >= len(proof.timeline.Keys) {
		return errCopiedHLSIndex
	}
	pts, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
	size, sizeErr := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
	if err != nil || sizeErr != nil || size <= 0 || math.Abs(float64(pts)*proof.base-proof.timeline.point(proof.count)) > max(0.000001, proof.timeline.TimeBase) {
		return errCopiedHLSIndex
	}
	proof.count++
	return nil
}

func certifyH264Configuration(ctx context.Context, logger *slog.Logger, ffmpeg, path string, timeline *h264Timeline) error {
	proof := copiedHLSConfigurationProof{maximum: len(timeline.Keys)}
	arguments := copiedHLSCertificationArguments(path, timeline, "h264_mp4toannexb,filter_units=pass_types=7|8")
	err := ProbeLines(ctx, logger, ffmpeg, arguments, 1<<20, maximumH264Keys+32, proof.line)
	if err != nil {
		return err
	}
	if proof.count == 0 {
		return errCopiedHLSIndex
	}
	return nil
}

func copiedHLSCertificationArguments(path string, timeline *h264Timeline, filter string) []string {
	arguments := []string{"-nostdin", "-v", "error", "-threads", "1", "-copyts"}
	if timeline.point(0) > 0 {
		arguments = append(arguments, "-seek_timestamp", "1", "-ss", h264Time(math.Floor(timeline.point(0)*1_000_000)/1_000_000))
	}
	arguments = append(arguments, "-i", path, "-map", "0:v:0", "-an", "-sn", "-dn", "-c:v", "copy", "-copytb", "1")
	if timeline.point(0) > 0 {
		arguments = append(arguments, "-copypriorss", "0")
	}
	return append(arguments, "-bsf:v", filter, "-f", "framehash", "pipe:1")
}

type copiedHLSConfigurationProof struct {
	maximum int
	first   string
	count   int
}

func (proof *copiedHLSConfigurationProof) line(line string) error {
	if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
		return nil
	}
	fields := strings.Split(line, ",")
	if len(fields) != 6 || proof.count >= proof.maximum {
		return errCopiedHLSIndex
	}
	size, sizeErr := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
	hash := strings.TrimSpace(fields[5])
	if sizeErr != nil || size <= 0 || len(hash) != 64 || proof.first != "" && hash != proof.first {
		return errCopiedHLSIndex
	}
	proof.first, proof.count = hash, proof.count+1
	return nil
}
