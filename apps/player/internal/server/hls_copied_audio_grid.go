package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"os/exec"
	"strconv"
)

var errCopiedHLSAudioGrid = fmt.Errorf("%w: audio grid", errCopiedHLSIndex)

type copiedHLSAudioGrid struct {
	Rate  int64
	Phase int64
}

type copiedHLSAudioPacket struct{ PTS, Duration *int64 }

type copiedHLSAudioStream struct {
	Codec string `json:"codec_name"`
	Rate  string `json:"sample_rate"`
	Base  string `json:"time_base"`
}

func validCopiedHLSAudioGrid(grid *copiedHLSAudioGrid) bool {
	if grid == nil {
		return true
	}
	if grid.Phase < 0 || grid.Phase >= 1024 {
		return false
	}
	switch grid.Rate {
	case 7350, 8000, 11025, 12000, 16000, 22050, 24000, 32000, 44100, 48000, 64000, 88200, 96000:
		return true
	}
	return false
}

func (manager *hlsManager) measureCopiedHLSAudioGrid(ctx context.Context, root *os.Root) (*copiedHLSAudioGrid, error) {
	initialization, err := copiedHLSCacheFile(root, "init.mp4", 2<<20)
	if err != nil {
		return nil, errCopiedHLSAudioGrid
	}
	fragment, err := copiedHLSCacheFile(root, "segment-00000.m4s", 64<<20)
	if err != nil {
		return nil, errCopiedHLSAudioGrid
	}
	output := copiedHLSProbeOutput{}
	//nolint:gosec // Installation-configured probe reads bounded, rooted generated media.
	command := exec.CommandContext(ctx, manager.probe.executable, "-v", "error", "-threads", "1", "-select_streams", "a:0",
		"-read_intervals", "%+#8", "-show_packets", "-show_streams", "-show_entries",
		"packet=pts,duration:stream=codec_name,sample_rate,time_base:stream_tags=:stream_disposition=", "-of", "json", "pipe:0")
	command.Stdin = io.MultiReader(bytes.NewReader(initialization), bytes.NewReader(fragment))
	command.Stdout = &output
	if command.Run() != nil || ctx.Err() != nil {
		return nil, errCopiedHLSAudioGrid
	}
	grid, err := decodeCopiedHLSAudioGrid(output.Bytes())
	if err == nil && grid == nil {
		slog.DebugContext(ctx, "HLS copied audio grid", "request_id", requestActivityID(ctx), "playback_session", requestPlaybackSession(ctx), "outcome", "initial-fragment-has-no-audio")
	}
	return grid, err
}

func decodeCopiedHLSAudioGrid(data []byte) (*copiedHLSAudioGrid, error) {
	var facts struct {
		Packets *[]copiedHLSAudioPacket `json:"packets"`
		Streams *[]copiedHLSAudioStream `json:"streams"`
	}
	if json.Unmarshal(data, &facts) != nil || facts.Packets == nil || facts.Streams == nil || len(*facts.Streams) > 1 {
		return nil, errCopiedHLSAudioGrid
	}
	grid, err := copiedHLSAudioStreamGrid(*facts.Streams)
	if err != nil {
		return nil, err
	}
	if len(*facts.Packets) == 0 {
		return nil, nil
	}
	if grid.Rate == 0 || len(*facts.Packets) > 8 {
		return nil, errCopiedHLSAudioGrid
	}
	grid.Phase, err = copiedHLSAudioPacketPhase(*facts.Packets)
	return grid, err
}

func copiedHLSAudioStreamGrid(streams []copiedHLSAudioStream) (*copiedHLSAudioGrid, error) {
	grid := &copiedHLSAudioGrid{}
	if len(streams) == 0 {
		return grid, nil
	}
	stream := streams[0]
	rate, err := strconv.ParseInt(stream.Rate, 10, 64)
	grid.Rate = rate
	if err != nil || stream.Codec != "aac" || stream.Base != "1/"+stream.Rate || !validCopiedHLSAudioGrid(grid) {
		return nil, errCopiedHLSAudioGrid
	}
	return grid, nil
}

func copiedHLSAudioPacketPhase(packets []copiedHLSAudioPacket) (int64, error) {
	var first int64
	for number, packet := range packets {
		if packet.PTS == nil || packet.Duration == nil || *packet.Duration != 1024 || *packet.PTS < -1024 || *packet.PTS > 1<<52 {
			return 0, errCopiedHLSAudioGrid
		}
		phase := (*packet.PTS%1024 + 1024) % 1024
		if number == 0 {
			first = phase
		} else if phase != first || *packet.PTS != *packets[number-1].PTS+1024 {
			return 0, errCopiedHLSAudioGrid
		}
	}
	return first, nil
}

func copiedHLSWarmAudio(timeline *copiedHLSTimeline, number int, mode string) bool {
	return timeline != nil && timeline.AudioSource != nil && timeline.AudioGrid != nil && mode == "audio-transcode" && number > 0 && !copiedHLSAudioFromOrigin(timeline, number, mode)
}

func copiedHLSWarmStart(timeline *copiedHLSTimeline, number int) float64 {
	origin, target, grid := timeline.point(0), timeline.point(number), timeline.AudioGrid
	if target-origin <= 3 {
		return origin
	}
	phase := grid.Phase - copiedHLSAudioClockOffset(timeline)
	limit := int64(math.Floor((target - 3) * float64(grid.Rate)))
	samples := limit - ((limit-phase)%1024+1024)%1024
	return max(origin, float64(samples)/float64(grid.Rate))
}

func copiedHLSWarmArguments(arguments []string, timeline *copiedHLSTimeline, number int) ([]string, error) {
	key, grid := timeline.Keys[number], timeline.AudioGrid
	if timeline.Clock == nil || !validCopiedHLSAudioGrid(grid) || !copiedHLSAudioTicksValid(timeline.Keys[0], key, timeline.Denominator) ||
		key.DTS > key.PTS || float64(key.PTS-key.DTS)*timeline.TimeBase > 1 {
		return nil, errCopiedHLSAudioGrid
	}
	micros := int64(math.Floor(copiedHLSWarmStart(timeline, number) * 1_000_000))
	seconds, remainder := micros/1_000_000, micros%1_000_000
	ticks := seconds * timeline.Denominator
	fraction := ticks%timeline.Numerator*1_000_000 + remainder*timeline.Denominator
	divisor := timeline.Numerator * 1_000_000
	videoSeek := ticks/timeline.Numerator + (2*fraction+divisor)/(2*divisor)
	audioSeek := copiedHLSAudioSeekSamples(micros, grid.Rate)
	dts := key.DTS * timeline.Numerator
	whole := dts/timeline.Denominator*grid.Rate - audioSeek
	part := dts % timeline.Denominator * grid.Rate
	cut := strconv.FormatInt
	audio := cut(whole, 10) + "+" + cut(part, 10) + "/" + cut(timeline.Denominator, 10)
	offset := float64(audioSeek+copiedHLSAudioClockOffset(timeline)) / float64(grid.Rate)
	return append(arguments, "-copypriorss", "0", "-avoid_negative_ts", "disabled", "-output_ts_offset", copiedHLSTime(offset),
		"-bsf:v", "noise=amount=0:drop=lt(pts\\,"+cut(key.PTS-videoSeek, 10)+")",
		"-bsf:a", "noise=amount=0:drop=lt(pts\\,"+audio+")"), nil
}

func copiedHLSAudioClockOffset(timeline *copiedHLSTimeline) int64 {
	clock, _ := copiedHLSAudioClockSamples(timeline)
	micros := int64(math.Floor(timeline.point(0) * 1_000_000))
	inputSamples := copiedHLSAudioSeekSamples(micros, timeline.AudioGrid.Rate)
	return clock - inputSamples
}

func copiedHLSAudioClockSamples(timeline *copiedHLSTimeline) (int64, bool) {
	grid := timeline.AudioGrid
	micros := int64(math.Round(*timeline.Clock * 1_000_000))
	clock := copiedHLSAudioSeekSamples(micros, grid.Rate)
	delta := (grid.Phase-clock%1024+1536)%1024 - 512
	bound := int64(math.Ceil(timeline.TimeBase*float64(grid.Rate))) + 2
	return clock + delta, delta >= -bound && delta <= bound
}

func copiedHLSAudioSeekSamples(micros, rate int64) int64 {
	return (micros*rate + 500_000) / 1_000_000
}

func copiedHLSClockFailureClass(result error) string {
	if errors.Is(result, errCopiedHLSAudioGrid) {
		return "audio-grid"
	}
	return "generation-or-assets"
}
