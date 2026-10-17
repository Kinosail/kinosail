package mediaprobe

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func longSubtitleFixture(b *testing.B) (string, *Probe, library.Item, []int) {
	b.Helper()
	ffmpeg, ffprobe := realSubtitleTools(b)
	directory := filepath.Join(os.TempDir(), "kinosail-embedded-subtitle-benchmark-v1")
	media := filepath.Join(directory, "film.mkv")
	if _, err := os.Stat(media); err != nil {
		staging, stagingErr := os.MkdirTemp(os.TempDir(), "kinosail-embedded-subtitle-benchmark-")
		if stagingErr != nil {
			b.Fatal(stagingErr)
		}
		writeMultiSubtitleFixture(b, ffmpeg, staging, 7200)
		if err = os.Rename(staging, directory); err != nil {
			b.Fatal(err)
		}
	}
	info, err := os.Stat(media)
	if err != nil {
		b.Fatal(err)
	}
	item := library.Item{ID: "film", Kind: "video", Path: media, Size: info.Size(), Added: info.ModTime()}
	probe := New(ffprobe)
	var streams []int
	for _, subtitle := range probe.Facts(b.Context(), item).SubtitleFacts {
		if subtitle.Text {
			streams = append(streams, subtitle.SourceIndex)
		}
	}
	if len(streams) != 4 {
		b.Fatalf("fixture text streams = %v", streams)
	}
	return ffmpeg, probe, item, streams
}

func BenchmarkEmbeddedFirstTrack(b *testing.B) {
	for _, allTextStreams := range []bool{false, true} {
		b.Run("allTextStreams="+strconv.FormatBool(allTextStreams), func(b *testing.B) {
			ffmpeg, probe, item, streams := longSubtitleFixture(b)
			root := b.TempDir()
			iteration := 0
			for b.Loop() {
				iteration++
				options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: filepath.Join(root, strconv.Itoa(iteration)), AllTextStreams: allTextStreams}
				if _, err := probe.Embedded(b.Context(), item, streams[0], options); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkEmbeddedEveryTrack(b *testing.B) {
	for _, prepared := range []bool{false, true} {
		b.Run("prepared="+strconv.FormatBool(prepared), func(b *testing.B) {
			ffmpeg, probe, item, streams := longSubtitleFixture(b)
			root := b.TempDir()
			iteration := 0
			for b.Loop() {
				iteration++
				options := EmbeddedOptions{FFmpeg: ffmpeg, CacheDir: filepath.Join(root, strconv.Itoa(iteration))}
				for position, stream := range streams {
					options.AllTextStreams = prepared && position == 0
					if _, err := probe.Embedded(b.Context(), item, stream, options); err != nil {
						b.Fatal(err)
					}
				}
			}
		})
	}
}
