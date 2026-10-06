package server

import (
	"bufio"
	"context"
	"errors"
	"io"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

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

func (timeline copiedHLSTimeline) point(number int) float64 {
	return float64(timeline.Keys[number].PTS) * timeline.TimeBase
}

func (manager *hlsManager) indexCopiedHLS(ctx context.Context, item library.Item, recipe hlsRecipe, preparation *startupEncoding) (*copiedHLSTimeline, error) {
	facts := manager.probe.facts(ctx, item)
	if recipe.mode != "remux" || facts.Video.Codec != "h264" || len(recipe.omitted) != 0 || !manager.index.Safe(item.Path) {
		return nil, errCopiedHLSIndex
	}
	options, err := manager.hlsSettings(item, recipe)
	if err != nil {
		return nil, err
	}
	scan, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(25 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-scan.Done():
				return
			case <-ticker.C:
				if preparation.adopted.Load() {
					cancel()
					return
				}
			}
		}
	}()
	release, err := manager.workloads.Acquire(scan, workload.Background)
	if err != nil {
		return nil, err
	}
	defer release()
	timeline := &copiedHLSTimeline{Policy: options.Cache, Strategy: "h264-idr-keys-1"}
	var end int64
	arguments := []string{"-v", "error", "-threads", "1", "-select_streams", "v:0", "-show_packets", "-show_streams",
		"-show_entries", "packet=pts,dts,duration,flags:stream=time_base", "-of", "compact=p=0", item.Path}
	err = copiedHLSLines(scan, manager.probe.executable, arguments, 128<<20, 2_000_010, func(line string) error {
		return timeline.packet(line, &end)
	})
	timeline.End = float64(end) * timeline.TimeBase
	if err != nil || !validCopiedHLSTimeline(timeline) {
		return nil, errCopiedHLSIndex
	}
	if timeline.End <= timeline.point(len(timeline.Keys)-1) || math.Abs(timeline.End-facts.Duration) > 0.1 {
		return nil, errCopiedHLSIndex
	}
	if err = manager.certifyCopiedHLS(scan, item, timeline); err != nil {
		return nil, err
	}
	if err = manager.certifyCopiedHLSConfiguration(scan, item, timeline); err != nil {
		return nil, err
	}
	first := -1
	for number := range timeline.Keys {
		if math.Abs(timeline.point(number)-recipe.offset) <= 0.000001 {
			first = number
			break
		}
	}
	if first < 0 {
		return nil, errCopiedHLSIndex
	}
	timeline.Keys = timeline.Keys[first:]
	if preparation.adopted.Load() || manager.validateHLSPolicy(scan, item, recipe, options.Cache) != nil {
		return nil, errCopiedHLSIndex
	}
	manager.mu.Lock()
	idle := len(manager.jobs) == 0
	manager.mu.Unlock()
	if !idle {
		return nil, errCopiedHLSIndex
	}
	return timeline, nil
}

func (timeline *copiedHLSTimeline) packet(line string, end *int64) error {
	fields := copiedHLSFields(line)
	if value, ok := fields["time_base"]; ok {
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
	pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
	duration, durationErr := strconv.ParseInt(fields["duration"], 10, 64)
	dts, dtsErr := strconv.ParseInt(fields["dts"], 10, 64)
	if ptsErr != nil || durationErr != nil || pts < 0 || pts > 1<<52 || duration <= 0 || duration > 1<<40 {
		return errCopiedHLSIndex
	}
	if strings.Contains(fields["flags"], "K") {
		if len(timeline.Keys) >= maximumCopiedHLSKeys || len(timeline.Keys) > 0 && pts <= timeline.Keys[len(timeline.Keys)-1].PTS {
			return errCopiedHLSIndex
		}
		if dtsErr != nil && len(timeline.Keys) > 0 {
			return errCopiedHLSIndex
		}
		timeline.Keys = append(timeline.Keys, copiedHLSKey{PTS: pts, DTS: dts})
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

// A key flag alone may be a recovery point. Filtered IDR packets must match the
// complete demux key map in order; uncertain boundaries cannot be prepared.
func (manager *hlsManager) certifyCopiedHLS(ctx context.Context, item library.Item, timeline *copiedHLSTimeline) error {
	base, count := 0.0, 0
	arguments := []string{"-nostdin", "-v", "error", "-threads", "1", "-copyts", "-i", item.Path, "-map", "0:v:0",
		"-an", "-sn", "-dn", "-c:v", "copy", "-copytb", "1", "-bsf:v", "filter_units=pass_types=5",
		"-f", "framehash", "pipe:1"}
	err := copiedHLSLines(ctx, manager.ffmpeg, arguments, 1<<20, maximumCopiedHLSKeys+32, func(line string) error {
		if strings.HasPrefix(line, "#tb 0: ") {
			fields := copiedHLSFields("time_base=" + strings.TrimPrefix(line, "#tb 0: "))
			probe := copiedHLSTimeline{}
			end := int64(0)
			if err := probe.packet("time_base="+fields["time_base"], &end); err != nil {
				return err
			}
			base = probe.TimeBase
			return nil
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			return nil
		}
		fields := strings.Split(line, ",")
		if len(fields) != 6 || base <= 0 || count >= len(timeline.Keys) {
			return errCopiedHLSIndex
		}
		pts, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		size, sizeErr := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
		if err != nil || sizeErr != nil || size <= 0 || math.Abs(float64(pts)*base-timeline.point(count)) > max(0.000001, timeline.TimeBase) {
			return errCopiedHLSIndex
		}
		count++
		return nil
	})
	if err != nil || count != len(timeline.Keys) {
		return errCopiedHLSIndex
	}
	return nil
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
	scanner := bufio.NewScanner(io.LimitReader(output, maximumBytes+1))
	scanner.Buffer(make([]byte, 1024), 1024)
	var total int64
	count := 0
	for scanner.Scan() {
		total += int64(len(scanner.Bytes()) + 1)
		count++
		if total > maximumBytes || count > maximumLines {
			err = errCopiedHLSIndex
			break
		}
		if err = visit(scanner.Text()); err != nil {
			break
		}
	}
	if scanner.Err() != nil {
		err = errCopiedHLSIndex
	}
	if err != nil {
		cancel()
	}
	if waitErr := command.Wait(); waitErr != nil || ctx.Err() != nil {
		err = errCopiedHLSIndex
	}
	return err
}

func (manager *hlsManager) certifyCopiedHLSConfiguration(ctx context.Context, item library.Item, timeline *copiedHLSTimeline) error {
	first, count := "", 0
	arguments := []string{"-nostdin", "-v", "error", "-threads", "1", "-copyts", "-i", item.Path,
		"-map", "0:v:0", "-an", "-sn", "-dn", "-c:v", "copy", "-copytb", "1",
		"-bsf:v", "h264_mp4toannexb,filter_units=pass_types=7|8", "-f", "framehash", "pipe:1"}
	err := copiedHLSLines(ctx, manager.ffmpeg, arguments, 1<<20, maximumCopiedHLSKeys+32, func(line string) error {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			return nil
		}
		fields := strings.Split(line, ",")
		if len(fields) != 6 || count >= len(timeline.Keys) {
			return errCopiedHLSIndex
		}
		size, sizeErr := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64)
		hash := strings.TrimSpace(fields[5])
		if sizeErr != nil || size <= 0 || len(hash) != 64 || first != "" && hash != first {
			return errCopiedHLSIndex
		}
		first, count = hash, count+1
		return nil
	})
	if err != nil || count == 0 {
		return errCopiedHLSIndex
	}
	return nil
}
