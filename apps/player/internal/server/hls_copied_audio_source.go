package server

import (
	"context"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

// The complete decoded source must have one sample clock before a seek may
// reconstruct that clock. A gap, format change or incomplete scan opts out.
type copiedHLSSourceAudio struct {
	Rate    int64
	Quantum int64
	Priming int64
	End     float64
}

type copiedHLSAudioScan struct {
	clock       *copiedHLSSourceAudio
	denominator int64
	frames      int
	samples     int64
	previous    int64
	format      string
	phaseLow    int64
	phaseHigh   int64
	rejected    bool
}

func validCopiedHLSSourceAudio(source *copiedHLSSourceAudio, end float64) bool {
	if source == nil {
		return true
	}
	if !validCopiedHLSAudioGrid(&copiedHLSAudioGrid{Rate: source.Rate}) || !validCopiedHLSEnd(source.End) || math.Abs(source.End-end) > 1 {
		return false
	}
	if source.Quantum == 0 {
		return source.Priming == 0
	}
	return source.Quantum == 1536 && source.Priming >= 0 && source.Priming < source.Quantum &&
		copiedHLSAC3Rate(source.Rate)
}

func copiedHLSAC3Rate(rate int64) bool {
	return rate == 32000 || rate == 44100 || rate == 48000
}

func (manager *hlsManager) indexCopiedHLSAudio(ctx context.Context, item library.Item, recipe hlsRecipe, timeline *copiedHLSTimeline) error {
	if recipe.mode != "audio-transcode" || timeline.End-timeline.point(0) <= maximumCopiedHLSAudioReplay {
		return nil
	}
	scan := copiedHLSAudioScan{}
	proof, cancel := copiedHLSAudioProofContext(ctx)
	defer cancel()
	arguments := []string{
		"-v", "error", "-threads", "1", "-select_streams", "a:" + strconv.Itoa(recipe.audio),
		"-show_streams", "-show_entries", "stream=codec_name,sample_rate,time_base,start_pts:stream_tags=:stream_disposition=", "-of", "compact=p=0", item.Path,
	}
	result := copiedHLSLines(proof, manager.probe.executable, arguments, 8<<10, 1, scan.header)
	if result == nil && scan.clock != nil {
		arguments = []string{
			"-v", "error", "-threads", "1", "-select_streams", "a:" + strconv.Itoa(recipe.audio),
			"-show_frames", "-show_entries", "frame=pts,nb_samples,channels,channel_layout,sample_fmt:frame_side_data=", "-of", "compact=p=0", item.Path,
		}
		result = copiedHLSLines(proof, manager.probe.executable, arguments, 192<<20, 2_000_000, scan.frame)
	}
	if ctx.Err() != nil || manager.validateHLSPolicy(ctx, item, recipe, timeline.Policy) != nil {
		return errCopiedHLSIndex
	}
	if result == nil && scan.complete(timeline.End) {
		timeline.AudioSource = scan.clock
		slog.DebugContext(ctx, "HLS audio source clock", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "outcome", "certified", "frames", scan.frames)
		return nil
	}
	// Preserve the previous path for delayed, discontinuous or unsupported
	// sources. Never apply a guessed clock or conceal the verification gap.
	slog.WarnContext(ctx, "HLS audio source clock", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "outcome", "legacy-seek", "failure_class", "source-clock-not-certified")
	return nil
}

func copiedHLSAudioProofContext(parent context.Context) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(6 * time.Second)
	if preparation, ok := parent.Deadline(); ok && preparation.Add(-2*time.Second).Before(deadline) {
		deadline = preparation.Add(-2 * time.Second)
	}
	return context.WithDeadline(parent, deadline)
}

func (scan *copiedHLSAudioScan) header(line string) error {
	fields := copiedHLSFields(line)
	rate, base, valid := copiedHLSAudioSourceHeader(fields)
	if scan.clock != nil || !valid {
		scan.clock, scan.rejected = nil, true
		return nil
	}
	scan.clock = &copiedHLSSourceAudio{Rate: rate}
	scan.denominator = base
	if base%rate != 0 {
		// Coarser clocks cannot identify a unique AC3 block after seeking.
		if base < 1000 || fields["codec_name"] != "ac3" || !copiedHLSAC3Rate(rate) {
			scan.clock, scan.rejected = nil, true
			return nil
		}
		scan.clock.Quantum = 1536
		scan.phaseHigh = rate - 1
	}
	return nil
}

func copiedHLSAudioSourceHeader(fields map[string]string) (int64, int64, bool) {
	rate, rateErr := strconv.ParseInt(fields["sample_rate"], 10, 32)
	numerator, denominator, found := strings.Cut(fields["time_base"], "/")
	base, baseErr := strconv.ParseInt(denominator, 10, 32)
	valid := rateErr == nil && baseErr == nil && found && numerator == "1" && base > 0 && fields["start_pts"] == "0" &&
		validCopiedHLSAudioGrid(&copiedHLSAudioGrid{Rate: rate})
	return rate, base, valid
}

func (scan *copiedHLSAudioScan) frame(line string) error {
	// An unsupported clock is a normal opt-out. Drain within the optional
	// proof budget so shared process checks still require genuine clean EOF.
	if scan.rejected {
		return nil
	}
	fields := copiedHLSFields(line)
	pts, ptsErr := strconv.ParseInt(fields["pts"], 10, 64)
	samples, samplesErr := strconv.ParseInt(fields["nb_samples"], 10, 32)
	if ptsErr != nil || samplesErr != nil || samples <= 0 || samples > 65536 || scan.samples > math.MaxUint32-samples {
		scan.rejected = true
		return nil //nolint:nilerr // Invalid source metadata opts out while the owned scanner drains.
	}
	if !scan.framePTS(pts) || !scan.frameShape(fields, samples) {
		scan.rejected = true
		return nil
	}
	scan.samples += samples
	scan.previous = samples
	scan.frames++
	return nil
}

func (scan *copiedHLSAudioScan) framePTS(pts int64) bool {
	rate, ticks := scan.clock.Rate, scan.samples*scan.denominator
	if scan.clock.Quantum == 0 {
		return pts == (ticks+rate/2)/rate
	}
	// A remux can round before shifting the first timestamp to zero. Keep
	// one quantizer phase for the ENTIRE source, never a per-frame allowance.
	// Subtract the bounded whole part before multiplying untrusted PTS.
	whole, remainder := ticks/rate, ticks%rate
	if pts < whole || pts > whole+1 {
		return false
	}
	lower := (pts-whole)*rate - remainder
	scan.phaseLow = max(scan.phaseLow, lower)
	scan.phaseHigh = min(scan.phaseHigh, lower+rate-1)
	return scan.phaseLow <= scan.phaseHigh
}

func (scan *copiedHLSAudioScan) frameShape(fields map[string]string, samples int64) bool {
	shape, valid := copiedHLSAudioFrameShape(fields)
	if !valid {
		return false
	}
	if scan.frames == 0 {
		scan.format = shape
		if scan.clock.Quantum != 0 {
			scan.clock.Priming = scan.clock.Quantum - samples
		}
	}
	if shape != scan.format || scan.clock.Quantum != 0 && samples > scan.clock.Quantum {
		return false
	}
	// Only the initial priming and the final partial frame may be short.
	return scan.clock.Quantum == 0 || scan.frames <= 1 || scan.previous == scan.clock.Quantum
}

func copiedHLSAudioFrameShape(fields map[string]string) (string, bool) {
	channels, err := strconv.ParseInt(fields["channels"], 10, 32)
	layout, format := fields["channel_layout"], fields["sample_fmt"]
	valid := err == nil && channels >= 1 && channels <= 32 && layout != "" && len(layout) <= 64 && format != "" && len(format) <= 16
	return fields["channels"] + ":" + layout + ":" + format, valid
}

func (scan *copiedHLSAudioScan) complete(end float64) bool {
	if scan.rejected || scan.clock == nil || scan.frames < 3 {
		return false
	}
	scan.clock.End = float64(scan.samples) / float64(scan.clock.Rate)
	return validCopiedHLSSourceAudio(scan.clock, end)
}

func copiedHLSSourceAudioArguments(arguments []string, timeline *copiedHLSTimeline, start float64, mode string) []string {
	if timeline == nil || timeline.AudioSource == nil || mode != "audio-transcode" {
		return arguments
	}
	// Keep a finite interleave window. Low-rate AAC can queue more than ten
	// seconds before producing packets; forced flushes misassign HLS audio.
	arguments = append(arguments, "-max_interleave_delta", "30000000")
	source := timeline.AudioSource
	if source.Quantum == 0 {
		return arguments
	}
	seek := copiedHLSInputTime(start)
	micros := int64(math.Floor(start * 1_000_000))
	target := copiedHLSAudioSeekSamples(micros, source.Rate)
	integer := func(value int64) string { return strconv.FormatInt(value, 10) }
	rate, quantum, priming := integer(source.Rate), integer(source.Quantum), integer(source.Priming)
	// Reconstruct BEFORE trimming. Adjusting encoded PTS cannot repair samples
	// already discarded by accurate seek against a coarse source clock.
	phase := "if(eq(N\\,0)\\,NB_SAMPLES-" + integer(source.Quantum-source.Priming) + "\\," + priming + ")"
	filter := "asettb=1/" + rate + ",asetpts=round(((PTS*TB+" + seek + ")*" + rate + "+" + phase + ")/" + quantum + ")*" + quantum + "-" + phase + "-" + integer(target) + ",atrim=start_pts=0,asetpts=PTS-STARTPTS"
	return append(arguments, "-af", filter)
}
