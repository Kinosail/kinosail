package server

import (
	"context"
	"errors"
	"math"
	"os/exec"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/workload"
)

const maximumCopiedHLSKeys = 4096

var errCopiedHLSIndex = errors.New("copied HLS timeline is unavailable")

type copiedHLSKey struct {
	PTS int64
	DTS int64
}

type copiedHLSTimeline struct {
	Policy      string
	Strategy    string
	Numerator   int64
	Denominator int64
	TimeBase    float64
	Keys        []copiedHLSKey
	End         float64
	Clock       *float64
}

func (timeline *copiedHLSTimeline) point(number int) float64 {
	return float64(timeline.Keys[number].PTS) * timeline.TimeBase
}

func (manager *hlsManager) indexCopiedHLS(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) (*copiedHLSTimeline, error) {
	facts := manager.probe.facts(ctx, item)
	if !manager.copiedHLSVideo(ctx, item, recipe) || facts.Video.Codec != "h264" || len(recipe.omitted) != 0 || !manager.index.Safe(item.Path) {
		return nil, errCopiedHLSIndex
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	scan, cancel := context.WithCancel(ctx)
	defer cancel()
	stopWatching := watchCopiedHLSAdoption(scan, preparation, cancel)
	defer stopWatching()
	release, err := manager.workloads.Acquire(scan, workload.Background)
	if err != nil {
		return nil, err
	}
	defer release()
	timeline, err := manager.scanCopiedHLSPackets(scan, item, options.Cache, facts.Duration)
	if err != nil {
		return nil, err
	}
	if err = manager.certifyCopiedHLS(scan, item, timeline); err != nil {
		return nil, err
	}
	if err = manager.certifyCopiedHLSConfiguration(scan, item, timeline); err != nil {
		return nil, err
	}
	return manager.selectCopiedHLSTimeline(scan, item, recipe, preparation, timeline)
}

func (timeline *copiedHLSTimeline) packet(line string, end *int64) error {
	fields := copiedHLSFields(line)
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

func copiedHLSFields(line string) map[string]string {
	fields := make(map[string]string)
	for _, field := range strings.Split(line, "|") {
		if key, value, ok := strings.Cut(field, "="); ok {
			fields[key] = value
		}
	}
	return fields
}

func copiedHLSLines(parent context.Context, executable string, arguments []string, maximumBytes int64, maximumLines int, visit func(string) error) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	//nolint:gosec // Executable is installation config; media comes from a revalidated scanned item.
	command := exec.CommandContext(ctx, executable, arguments...)
	output, err := command.StdoutPipe()
	if err != nil {
		return errCopiedHLSIndex
	}
	if command.Start() != nil {
		return errCopiedHLSIndex
	}
	err = scanCopiedHLSLines(output, maximumBytes, maximumLines, visit)
	if err != nil {
		cancel()
	}
	if waitErr := command.Wait(); waitErr != nil || ctx.Err() != nil {
		err = errCopiedHLSIndex
	}
	return err
}

func (manager *hlsManager) scanCopiedHLSPackets(ctx context.Context, item library.Item, policy string, duration float64) (*copiedHLSTimeline, error) {
	timeline := &copiedHLSTimeline{Policy: policy, Strategy: "h264-idr-keys-1"}
	var end int64
	arguments := []string{
		"-v", "error", "-threads", "1", "-select_streams", "v:0", "-show_packets", "-show_streams",
		"-show_entries", "packet=pts,dts,duration,flags:stream=time_base", "-of", "compact=p=0", item.Path,
	}
	err := copiedHLSLines(ctx, manager.probe.executable, arguments, 128<<20, 2_000_010, func(line string) error {
		return timeline.packet(line, &end)
	})
	timeline.End = float64(end) * timeline.TimeBase
	if err != nil || !validCopiedHLSTimeline(timeline) {
		return nil, errCopiedHLSIndex
	}
	if timeline.End <= timeline.point(len(timeline.Keys)-1) || math.Abs(timeline.End-duration) > 0.1 {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}

func validCopiedHLSPacket(pts, duration int64, ptsErr, durationErr error) bool {
	return ptsErr == nil && durationErr == nil && pts >= 0 && pts <= 1<<52 && duration > 0 && duration <= 1<<40
}

func (timeline *copiedHLSTimeline) recordKey(pts, dts int64, dtsErr error) error {
	if len(timeline.Keys) >= maximumCopiedHLSKeys || len(timeline.Keys) > 0 && pts <= timeline.Keys[len(timeline.Keys)-1].PTS {
		return errCopiedHLSIndex
	}
	if dtsErr != nil && len(timeline.Keys) > 0 {
		return errCopiedHLSIndex
	}
	timeline.Keys = append(timeline.Keys, copiedHLSKey{PTS: pts, DTS: dts})
	return nil
}
