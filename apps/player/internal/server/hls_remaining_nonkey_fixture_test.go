package server_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func remainingNonKeyGenerate(t *testing.T, ctx context.Context, ffmpeg, source string) {
	t.Helper()
	command := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error",
		"-f", "lavfi", "-i", "testsrc2=s=640x360:r=24:d=32",
		"-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=32",
		"-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-crf", "32",
		"-g", "48", "-keyint_min", "48", "-sc_threshold", "0", "-frames:v", "768",
		"-c:a", "aac", "-ac", "2", "-avoid_negative_ts", "disabled", source) //nolint:gosec // Fixed owned fixture and discovered pinned codec.
	if command.Run() != nil {
		t.Fatal("nonkey fixture generation failed")
	}
}

func remainingNonKeyQualify(t *testing.T, ctx context.Context, ffmpeg, ffprobe, source string) {
	t.Helper()
	remainingNonKeyQualifyStreams(t, ctx, ffprobe, source)
	remainingNonKeyQualifyVideoPackets(t, ctx, ffprobe, source)
	remainingNonKeyIDRs(t, ctx, ffmpeg, source)
}

func remainingNonKeyQualifyStreams(t *testing.T, ctx context.Context, ffprobe, source string) {
	t.Helper()
	output, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-count_frames",
		"-show_entries", "stream=codec_name,codec_type,nb_read_frames,channels,sample_rate:format=duration",
		"-of", "json", source).Output() //nolint:gosec // Owned bounded fixture and pinned probe.
	if err != nil || len(output) > 32<<10 {
		t.Fatal("nonkey fixture stream probe bound")
	}
	var facts struct {
		Streams []remainingNonKeyFixtureStream
		Format  struct{ Duration string }
	}
	if json.Unmarshal(output, &facts) != nil {
		t.Fatal("nonkey fixture stream JSON")
	}
	video, audio := remainingNonKeyFixtureStreams(facts.Streams)
	duration, err := strconv.ParseFloat(facts.Format.Duration, 64)
	if !video || !audio || err != nil || duration < 31.95 || duration > 32.05 {
		t.Fatal("nonkey fixture codec/frame/EOF qualification")
	}
}

type remainingNonKeyFixtureStream struct {
	Codec    string `json:"codec_name"`
	Kind     string `json:"codec_type"`
	Frames   string `json:"nb_read_frames"`
	Channels int    `json:"channels"`
	Rate     string `json:"sample_rate"`
}

func remainingNonKeyFixtureStreams(streams []remainingNonKeyFixtureStream) (bool, bool) {
	video, audio := false, false
	for _, stream := range streams {
		video = video || stream.Kind == "video" && stream.Codec == "h264" && stream.Frames == "768"
		audio = audio || stream.Kind == "audio" && stream.Codec == "aac" && stream.Channels == 2 && stream.Rate == "48000"
	}
	return video, audio
}

func remainingNonKeyQualifyVideoPackets(t *testing.T, ctx context.Context, ffprobe, source string) {
	t.Helper()
	output, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-select_streams", "v:0",
		"-show_packets", "-show_entries", "packet=pts_time,flags", "-of", "json", source).Output() //nolint:gosec // Owned bounded fixture and pinned probe.
	if err != nil || len(output) > 512<<10 {
		t.Fatal("nonkey fixture packet probe bound")
	}
	var packets struct {
		Packets []struct {
			PTS   string `json:"pts_time"`
			Flags string `json:"flags"`
		}
	}
	if json.Unmarshal(output, &packets) != nil || len(packets.Packets) != 768 {
		t.Fatal("nonkey fixture complete video packet rows")
	}
	keys := 0
	for _, packet := range packets.Packets {
		if !strings.Contains(packet.Flags, "K") {
			continue
		}
		point, err := strconv.ParseFloat(packet.PTS, 64)
		if err != nil || point != float64(keys*2) {
			t.Fatal("nonkey fixture independent two-second source keys")
		}
		keys++
	}
	if keys != 16 {
		t.Fatal("nonkey fixture independent key count")
	}
}

func remainingNonKeyIDRs(t *testing.T, ctx context.Context, ffmpeg, source string) {
	t.Helper()
	output, err := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error",
		"-threads", "1", "-copyts", "-i", source, "-map", "0:v:0", "-an", "-sn", "-dn",
		"-c:v", "copy", "-copytb", "1", "-bsf:v", "filter_units=pass_types=5",
		"-f", "framehash", "pipe:1").Output() //nolint:gosec // Fixed IDR filter on owned bounded fixture.
	if err != nil || len(output) > 32<<10 {
		t.Fatal("nonkey fixture IDR certification bound")
	}
	base, count := float64(0), 0
	for _, line := range strings.Split(string(output), "\n") {
		if value, ok := strings.CutPrefix(line, "#tb 0: "); ok {
			base = remainingNonKeyIDRTimeBase(t, value)
			continue
		}
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		remainingNonKeyIDRPacket(t, line, base, count)
		count++
	}
	if count != 16 {
		t.Fatal("nonkey fixture complete IDR correspondence")
	}
}

func remainingNonKeyIDRTimeBase(t *testing.T, value string) float64 {
	t.Helper()
	a, b, found := strings.Cut(value, "/")
	numerator, aErr := strconv.ParseFloat(a, 64)
	denominator, bErr := strconv.ParseFloat(b, 64)
	if !found || aErr != nil || bErr != nil || numerator <= 0 || denominator <= 0 {
		t.Fatal("nonkey fixture IDR time base")
	}
	return numerator / denominator
}

func remainingNonKeyIDRPacket(t *testing.T, line string, base float64, count int) {
	t.Helper()
	fields := strings.Split(line, ",")
	if len(fields) != 6 || count >= 16 || base <= 0 {
		t.Fatal("nonkey fixture IDR packet structure")
	}
	pts, ptsErr := strconv.ParseFloat(strings.TrimSpace(fields[2]), 64)
	size, sizeErr := strconv.Atoi(strings.TrimSpace(fields[4]))
	if ptsErr != nil || sizeErr != nil || size <= 0 || pts*base != float64(count*2) {
		t.Fatal("nonkey fixture IDR correspondence")
	}
}
