package mediaprobe

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"syscall"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

type probeOutcome int

const (
	probeRejected probeOutcome = iota
	probeFound
	probeBroken
	probeInterrupted
)

func (probe *Probe) run(ctx context.Context, item library.Item) (Result, probeOutcome) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	data, err := runProbe(ctx, probe.executable, "-v", "error", "-show_entries", "stream=index,codec_type,codec_name,profile,level,pix_fmt,field_order,width,height,r_frame_rate,sample_aspect_ratio,sample_rate,channels,channel_layout,color_range,color_space,color_transfer,color_primaries,bits_per_raw_sample:stream_disposition:stream_tags=language,title:stream_side_data:chapter=start_time,end_time:chapter_tags=title:format=format_name,duration,bit_rate:format_tags=title,artist,album_artist,album,genre,track,disc,replaygain_track_gain,replaygain_album_gain,r128_track_gain,r128_album_gain", "-of", "json", item.Path)
	if err != nil {
		if probeInterruptedBy(ctx, err) {
			return Result{}, probeInterrupted
		}
		return Result{}, probeBroken
	}
	if result, found := parse(data); found {
		return result, probeFound
	}
	return Result{}, probeRejected
}

func logProbeFailure(ctx context.Context, item library.Item, outcome probeOutcome) {
	if ctx.Err() != nil || outcome == probeRejected {
		return
	}
	label := "broken"
	if outcome == probeInterrupted {
		label = "interrupted"
	}
	slog.WarnContext(ctx, "media probe failed", "operation", "mediaprobe.facts", "item_id", item.ID, "outcome", label, "backoff", outcome == probeBroken)
}

func probeInterruptedBy(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && (status.Signal() == syscall.SIGKILL || status.Signal() == syscall.SIGTERM || status.Signal() == syscall.SIGINT)
}
