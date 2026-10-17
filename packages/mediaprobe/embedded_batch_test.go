package mediaprobe

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func realSubtitleTools(tb testing.TB) (string, string) {
	tb.Helper()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		tb.Skip("FFmpeg is required for the embedded subtitle extraction test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		tb.Skip("FFprobe is required for the embedded subtitle extraction test")
	}
	return ffmpeg, ffprobe
}

func writeMultiSubtitleFixture(tb testing.TB, ffmpeg, root string, seconds int) library.Item {
	tb.Helper()
	languages := []string{"eng", "fra", "deu", "spa"}
	arguments := []string{"-v", "error", "-y", "-f", "lavfi", "-i", fmt.Sprintf("testsrc=size=160x90:rate=12:duration=%d", seconds), "-f", "lavfi", "-i", fmt.Sprintf("sine=duration=%d", seconds)}
	for position, language := range languages {
		var subtitle strings.Builder
		for cue := range seconds / 2 {
			start := cue * 2
			fmt.Fprintf(&subtitle, "%d\n00:%02d:%02d,000 --> 00:%02d:%02d,500\n%s cue %d <i>styled</i> & more\n\n", cue+1, start/60, start%60, start/60, start%60+1, language, position*1000+cue)
		}
		path := filepath.Join(root, language+".srt")
		if err := os.WriteFile(path, []byte(subtitle.String()), 0o600); err != nil {
			tb.Fatal(err)
		}
		arguments = append(arguments, "-i", path)
	}
	arguments = append(arguments, "-map", "0", "-map", "1")
	for position, language := range languages {
		codec := "srt"
		if position%2 == 1 {
			codec = "ass"
		}
		arguments = append(arguments, "-map", strconv.Itoa(position+2), fmt.Sprintf("-c:s:%d", position), codec, fmt.Sprintf("-metadata:s:s:%d", position), "language="+language)
	}
	media := filepath.Join(root, "film.mkv")
	arguments = append(arguments, "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", media)
	if output, err := exec.CommandContext(tb.Context(), ffmpeg, arguments...).CombinedOutput(); err != nil {
		tb.Fatalf("fixture generation failed: %v %s", err, output)
	}
	info, err := os.Stat(media)
	if err != nil {
		tb.Fatal(err)
	}
	return library.Item{ID: "film", Kind: "video", Path: media, Size: info.Size(), Added: info.ModTime()}
}

func textStreams(t *testing.T, probe *Probe, item library.Item) []int {
	t.Helper()
	var streams []int
	for _, subtitle := range probe.Facts(t.Context(), item).SubtitleFacts {
		if subtitle.Text {
			streams = append(streams, subtitle.SourceIndex)
		}
	}
	if len(streams) != 4 {
		t.Fatalf("fixture text streams = %v", streams)
	}
	return streams
}

func TestEmbeddedExtractsEveryTextStreamInOnePassWithIdenticalBytes(t *testing.T) {
	t.Parallel()
	ffmpeg, ffprobe := realSubtitleTools(t)
	root := t.TempDir()
	item := writeMultiSubtitleFixture(t, ffmpeg, root, 40)
	streams := textStreams(t, New(ffprobe), item)
	reference := make(map[int][]byte, len(streams))
	for _, stream := range streams {
		subtitle, err := New(ffprobe).Embedded(t.Context(), item, stream, EmbeddedOptions{FFmpeg: ffmpeg})
		if err != nil || subtitle.Path != "" || len(subtitle.Data) == 0 {
			t.Fatalf("single-stream extraction %d = %#v, %v", stream, subtitle, err)
		}
		reference[stream] = subtitle.Data
	}
	calls, counting := filepath.Join(root, "calls"), filepath.Join(root, "counting-ffmpeg")
	writeProbeScript(t, counting, "#!/bin/sh\nprintf x >> '"+calls+"'\nexec '"+ffmpeg+"' \"$@\"\n")
	probe, cache := New(ffprobe), filepath.Join(root, "cache")
	options := EmbeddedOptions{FFmpeg: counting, CacheDir: cache}
	for _, stream := range []int{streams[1], streams[0], streams[3], streams[2]} {
		subtitle, err := probe.Embedded(t.Context(), item, stream, options)
		if err != nil || subtitle.Path != embeddedSubtitlePath(cache, item, stream) {
			t.Fatalf("cached extraction %d = %#v, %v", stream, subtitle, err)
		}
		data, err := readEmbeddedSubtitle(subtitle.Path)
		if err != nil || !bytes.Equal(data, reference[stream]) {
			t.Fatalf("stream %d bytes differ from single-stream extraction: %v\n%q\n%q", stream, err, data, reference[stream])
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("FFmpeg passes = %q, %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Dir(embeddedSubtitlePath(cache, item, streams[0])))
	if err != nil || len(entries) != len(streams) {
		t.Fatalf("cache entries = %v, %v", entries, err)
	}
}

func TestEmbeddedFallsBackToSingleStreamWhenCombinedPassFails(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	probe := New("unused")
	probe.cache[item.ID] = Result{SubtitleFacts: []SubtitleFacts{
		{SourceIndex: 2, Codec: "subrip", Language: "eng", Text: true},
		{SourceIndex: 3, Codec: "hdmv_pgs_subtitle", Language: "eng"},
		{SourceIndex: 4, Codec: "ass", Language: "fra", Text: true},
	}}
	calls, ffmpeg, cache := filepath.Join(root, "calls"), filepath.Join(root, "ffmpeg"), filepath.Join(root, "cache")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nmaps=0\nfor argument in \"$@\"; do [ \"$argument\" = -map ] && maps=$((maps+1)); done\nprintf '%s ' \"$maps\" >> '"+calls+"'\n[ \"$maps\" -gt 1 ] && { printf partial > \"$(eval echo \\${$#})\"; exit 1; }\nprintf '%s' '"+testWebVTT+"'\n")
	subtitle, err := probe.Embedded(t.Context(), item, 4, EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache})
	if err != nil || subtitle.Path != embeddedSubtitlePath(cache, item, 4) {
		t.Fatalf("fallback extraction = %#v, %v", subtitle, err)
	}
	if data, readErr := readEmbeddedSubtitle(subtitle.Path); readErr != nil || string(data) != testWebVTT {
		t.Fatalf("fallback bytes = %q, %v", data, readErr)
	}
	if data, readErr := os.ReadFile(calls); readErr != nil || string(data) != "2 1 " {
		t.Fatalf("FFmpeg map counts = %q, %v", data, readErr)
	}
	entries, err := os.ReadDir(filepath.Dir(subtitle.Path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "4.vtt" {
		t.Fatalf("failed combined pass left cache entries %v, %v", entries, err)
	}
}

func TestEmbeddedCompanionRequestJoinsRunningCombinedPass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	probe := New("unused")
	probe.cache[item.ID] = Result{SubtitleFacts: []SubtitleFacts{{SourceIndex: 2, Text: true}, {SourceIndex: 4, Text: true}}}
	calls, release, ffmpeg := filepath.Join(root, "calls"), filepath.Join(root, "release"), filepath.Join(root, "ffmpeg")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nprintf x >> '"+calls+"'\nwhile [ ! -f '"+release+"' ]; do sleep 0.01; done\nfor argument in \"$@\"; do case \"$argument\" in file:*) printf '%s' '"+testWebVTT+"' > \"${argument#file:}\";; esac; done\n")
	options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: filepath.Join(root, "cache")}
	owner := make(chan error, 1)
	go func() { _, err := probe.Embedded(t.Context(), item, 2, options); owner <- err }()
	waitEmbeddedFile(t, calls, owner)
	companion := make(chan error, 1)
	go func() { _, err := probe.Embedded(t.Context(), item, 4, options); companion <- err }()
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, result := range []<-chan error{owner, companion} {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	if data, err := os.ReadFile(calls); err != nil || string(data) != "x" {
		t.Fatalf("FFmpeg passes = %q, %v", data, err)
	}
	for _, stream := range []int{2, 4} {
		if data, err := readEmbeddedSubtitle(embeddedSubtitlePath(options.CacheDir, item, stream)); err != nil || string(data) != testWebVTT {
			t.Fatalf("stream %d cache = %q, %v", stream, data, err)
		}
	}
}

func TestEmbeddedCombinedPassRejectsOversizedAndMissingOutputs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	item := library.Item{ID: "film", Path: filepath.Join(root, "film.mkv")}
	probe := New("unused")
	probe.cache[item.ID] = Result{SubtitleFacts: []SubtitleFacts{{SourceIndex: 2, Text: true}, {SourceIndex: 4, Text: true}, {SourceIndex: 5, Text: true}, {SourceIndex: 6, Text: true}}}
	ffmpeg, cache := filepath.Join(root, "ffmpeg"), filepath.Join(root, "cache")
	writeProbeScript(t, ffmpeg, "#!/bin/sh\nfor argument in \"$@\"; do case \"$argument\" in file:*/2.vtt) printf '%s' '"+testWebVTT+"' > \"${argument#file:}\";; file:*/4.vtt) head -c 16777217 /dev/zero > \"${argument#file:}\";; file:*/5.vtt) : > \"${argument#file:}\";; esac; done\n")
	options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: cache}
	if subtitle, err := probe.Embedded(t.Context(), item, 2, options); err != nil || subtitle.Path == "" {
		t.Fatalf("valid stream = %#v, %v", subtitle, err)
	}
	entries, err := os.ReadDir(filepath.Dir(embeddedSubtitlePath(cache, item, 2)))
	if err != nil || len(entries) != 1 || entries[0].Name() != "2.vtt" {
		t.Fatalf("rejected outputs were cached: %v, %v", entries, err)
	}
	for _, stream := range []int{4, 5, 6} {
		if _, err := probe.Embedded(t.Context(), item, stream, options); err == nil {
			t.Fatalf("stream %d without valid output succeeded", stream)
		}
	}
}

func BenchmarkEmbeddedAllTextStreams(b *testing.B) {
	ffmpeg, ffprobe := realSubtitleTools(b)
	root := b.TempDir()
	item := writeMultiSubtitleFixture(b, ffmpeg, root, 1200)
	probe := New(ffprobe)
	var streams []int
	for _, subtitle := range probe.Facts(b.Context(), item).SubtitleFacts {
		if subtitle.Text {
			streams = append(streams, subtitle.SourceIndex)
		}
	}
	iteration := 0
	for b.Loop() {
		iteration++
		options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: filepath.Join(root, "cache-"+strconv.Itoa(iteration))}
		for _, stream := range streams {
			if _, err := probe.Embedded(b.Context(), item, stream, options); err != nil {
				b.Fatal(err)
			}
		}
	}
}
