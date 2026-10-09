package server

import (
	"context"
	"errors"
	"io"
	"math"
	"os"
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
	Policy       string
	Strategy     string
	Numerator    int64
	Denominator  int64
	TimeBase     float64
	Keys         []copiedHLSKey
	End          float64
	Clock        *float64
	Presentation *copiedHLSPresentation `json:",omitempty"`
	AudioOrigin  *copiedHLSAudioOrigin  `json:",omitempty"`
}

func (timeline *copiedHLSTimeline) point(number int) float64 {
	return float64(timeline.Keys[number].PTS) * timeline.TimeBase
}

func (manager *hlsManager) indexCopiedHLS(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) (*copiedHLSTimeline, error) {
	facts := manager.probe.facts(ctx, item)
	if !manager.copiedHLSVideo(ctx, item, recipe) || facts.Video.Codec != "h264" || len(recipe.omitted) != 0 || !manager.index.Safe(item.Path) {
		return nil, errCopiedHLSIndex
	}
	if err := manager.qualifyCopiedAAC(ctx, item, recipe, true); err != nil {
		return nil, err
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
	var origin *copiedHLSAudioOrigin
	if copiedAACPolicyRequired(options.Cache) {
		source, err := os.Lstat(item.Path)
		selected, track, known := manager.copiedAACPolicy(hlsRecipeKey(item.ID, recipe), copiedAACBasePolicy(options.Cache), source)
		if err != nil || !selected || !known {
			return nil, errCopiedHLSIndex
		}
		origin = &copiedHLSAudioOrigin{SourceTrack: track}
	}
	timeline, err := manager.scanCopiedHLSPackets(scan, item, options.Cache, facts.Duration, origin)
	if err != nil {
		return nil, err
	}
	if err = manager.certifyCopiedHLS(scan, item, timeline); err != nil {
		return nil, err
	}
	if err = manager.certifyCopiedHLSConfiguration(scan, item, timeline); err != nil {
		return nil, err
	}
	selected, err := manager.selectCopiedHLSTimeline(scan, item, recipe, preparation, timeline)
	if err != nil {
		return nil, err
	}
	if copiedAACPolicyRequired(options.Cache) {
		initial, err := copiedAACKeyMicros(selected, selected.Keys[0].PTS)
		source, statErr := os.Lstat(item.Path)
		_, track, known := manager.copiedAACPolicy(hlsRecipeKey(item.ID, recipe), copiedAACBasePolicy(options.Cache), source)
		if err != nil || statErr != nil || !known {
			return nil, errCopiedHLSIndex
		}
		selected.AudioOrigin = &copiedHLSAudioOrigin{SourceTrack: track, InitialSeekMicros: initial}
	}
	return selected, nil
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
	return copiedHLSInputLines(parent, executable, arguments, nil, maximumBytes, maximumLines, visit)
}

func copiedHLSInputLines(parent context.Context, executable string, arguments []string, input io.Reader, maximumBytes int64, maximumLines int, visit func(string) error) error {
	ctx, cancel, err := copiedHLSProbeContext(parent)
	if err != nil {
		return err
	}
	defer cancel()
	//nolint:gosec // Executable is installation config; media comes from a revalidated scanned item.
	command := exec.CommandContext(ctx, executable, arguments...)
	command.Stdin = input
	command.Cancel = nil // The joined owner below settles the entire process group.
	probe, output, err := startCopiedHLSProbe(ctx, command)
	if err != nil {
		return errCopiedHLSIndex
	}
	defer probe.close()
	defer output.Close()
	scanned := make(chan error, 1)
	watched := make(chan copiedHLSProbeWatch, 1)
	go func() { watched <- watchCopiedHLSProbe(ctx, probe, output, scanned) }()
	err = scanCopiedHLSLines(output, maximumBytes, maximumLines, visit)
	scanned <- err
	watch := <-watched
	waitErr := probe.wait()
	settleErr := settleCopiedHLSProbe(parent, probe)
	watchErr := watch.completionError(ctx, err, waitErr, settleErr)
	if watchErr != nil || waitErr != nil || settleErr != nil || ctx.Err() != nil {
		if ctx.Err() == nil {
			reportCopiedHLSProbeCompletion(parent, err, watchErr, waitErr, settleErr)
		}
		return errCopiedHLSIndex
	}
	return err
}

func (manager *hlsManager) scanCopiedHLSPackets(ctx context.Context, item library.Item, policy string, duration float64, origins ...*copiedHLSAudioOrigin) (*copiedHLSTimeline, error) {
	timeline := &copiedHLSTimeline{Policy: policy, Strategy: "h264-idr-keys-1"}
	if len(origins) > 0 && origins[0] != nil {
		origin := *origins[0]
		timeline.AudioOrigin = &origin
	}
	var end int64
	arguments := []string{
		"-v", "error", "-threads", "1", "-select_streams", "v:0", "-show_packets", "-show_streams",
		"-show_entries", "packet=pts,dts,duration,flags:stream=time_base", "-of", "compact=p=0", item.Path,
	}
	err := copiedHLSLines(ctx, manager.probe.executable, arguments, 128<<20, 2_000_010, func(line string) error {
		return timeline.packet(line, &end)
	})
	if timeline.AudioOrigin != nil && len(timeline.Keys) > 0 {
		initial, rescaleErr := copiedAACKeyMicros(timeline, timeline.Keys[0].PTS)
		if rescaleErr != nil {
			return nil, errCopiedHLSIndex
		}
		timeline.AudioOrigin.InitialSeekMicros = initial
	}
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
