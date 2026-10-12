package server

import (
	"context"
	"errors"
	"log/slog"
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
	AudioGrid   *copiedHLSAudioGrid   `json:",omitempty"`
	AudioSource *copiedHLSSourceAudio `json:",omitempty"`
}

func (timeline *copiedHLSTimeline) point(number int) float64 {
	return float64(timeline.Keys[number].PTS) * timeline.TimeBase
}

func (manager *hlsManager) indexCopiedHLS(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) (*copiedHLSTimeline, error) {
	phase := "eligibility"
	defer func() {
		slog.DebugContext(ctx, "HLS copied index", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "phase", phase, "canceled", ctx.Err() != nil)
	}()
	facts := manager.probe.facts(ctx, item)
	if !manager.copiedHLSVideo(ctx, item, recipe) || facts.Video.Codec != "h264" || len(recipe.omitted) != 0 || !manager.index.Safe(item.Path) {
		return nil, errCopiedHLSIndex
	}
	phase = "settings"
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	scan, cancel := context.WithCancel(ctx)
	defer cancel()
	stopWatching := watchCopiedHLSAdoption(scan, preparation, cancel)
	defer stopWatching()

	phase = "admission"
	release, err := manager.workloads.Acquire(scan, workload.Background)
	if err != nil {
		return nil, err
	}
	defer release()

	phase = "packets"
	timeline, err := manager.scanCopiedHLSPackets(scan, item, options.Cache, facts.Duration)
	if err != nil {
		return nil, err
	}
	phase = "idr"
	if err = manager.certifyCopiedHLS(scan, item, timeline); err != nil {
		return nil, err
	}
	phase = "configuration"
	if err = manager.certifyCopiedHLSConfiguration(scan, item, timeline); err != nil {
		return nil, err
	}
	phase = "selection"
	selected, err := manager.selectCopiedHLSTimeline(scan, item, recipe, preparation, timeline)
	if err != nil {
		return nil, err
	}
	phase = "audio-clock"
	if err := manager.indexCopiedHLSAudio(scan, item, recipe, selected); err != nil {
		return nil, err
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
	ctx, cancel, err := copiedHLSProbeContext(parent)
	if err != nil {
		return err
	}
	defer cancel()
	//nolint:gosec // Executable is installation config; media comes from a revalidated scanned item.
	command := exec.CommandContext(ctx, executable, arguments...)
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

func (manager *hlsManager) scanCopiedHLSPackets(ctx context.Context, item library.Item, policy string, duration float64) (*copiedHLSTimeline, error) {
	timeline := &copiedHLSTimeline{Policy: policy, Strategy: "h264-idr-keys-1"}
	var end int64
	originKnown := false
	arguments := []string{
		"-v", "error", "-threads", "1", "-select_streams", "v:0", "-show_packets", "-show_streams", "-show_format",
		"-show_entries", "packet=pts,dts,duration,flags:stream=time_base:format=start_time:format_tags=:stream_tags=:stream_disposition=", "-of", "compact=p=0", item.Path,
	}
	err := copiedHLSLines(ctx, manager.probe.executable, arguments, 128<<20, 2_000_010, func(line string) error {
		if value, ok := copiedHLSFields(line)["start_time"]; ok {
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
	if err != nil || !originKnown || !validCopiedHLSTimeline(timeline) {
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
