package server

import (
	"context"
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

// A key flag alone may be a recovery point. Filtered IDR packets must match the
// complete demux key map in order; uncertain boundaries cannot be prepared.
func (manager *hlsManager) certifyCopiedHLS(ctx context.Context, item library.Item, timeline *copiedHLSTimeline) error {
	proof := copiedHLSIDRProof{timeline: timeline}
	arguments := []string{
		"-nostdin", "-v", "error", "-threads", "1", "-copyts", "-i", item.Path, "-map", "0:v:0",
		"-an", "-sn", "-dn", "-c:v", "copy", "-copytb", "1", "-bsf:v", "filter_units=pass_types=5",
		"-f", "framehash", "pipe:1",
	}
	err := copiedHLSLines(ctx, manager.ffmpeg, arguments, 1<<20, maximumCopiedHLSKeys+32, proof.line)
	if err != nil || proof.count != len(timeline.Keys) {
		return errCopiedHLSIndex
	}
	return nil
}

type copiedHLSIDRProof struct {
	timeline *copiedHLSTimeline
	base     float64
	count    int
}

func (proof *copiedHLSIDRProof) line(line string) error {
	if strings.HasPrefix(line, "#tb 0: ") {
		fields := copiedHLSFields("time_base=" + strings.TrimPrefix(line, "#tb 0: "))
		probe := copiedHLSTimeline{}
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

func (manager *hlsManager) certifyCopiedHLSConfiguration(ctx context.Context, item library.Item, timeline *copiedHLSTimeline) error {
	proof := copiedHLSConfigurationProof{maximum: len(timeline.Keys)}
	arguments := []string{
		"-nostdin", "-v", "error", "-threads", "1", "-copyts", "-i", item.Path,
		"-map", "0:v:0", "-an", "-sn", "-dn", "-c:v", "copy", "-copytb", "1",
		"-bsf:v", "h264_mp4toannexb,filter_units=pass_types=7|8", "-f", "framehash", "pipe:1",
	}
	err := copiedHLSLines(ctx, manager.ffmpeg, arguments, 1<<20, maximumCopiedHLSKeys+32, proof.line)
	if err != nil || proof.count == 0 {
		return errCopiedHLSIndex
	}
	return nil
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
