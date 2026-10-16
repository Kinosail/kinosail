package playback

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/servertest/mp4fixture"
)

func finishedHLSRendition(tb testing.TB, segments int) (string, string) {
	tb.Helper()
	root := tb.TempDir()
	source := filepath.Join(root, "movie.mkv")
	if err := os.WriteFile(source, []byte("source"), 0o600); err != nil {
		tb.Fatal(err)
	}
	directory := filepath.Join(root, "0123456789abcdef", "360p")
	if err := os.MkdirAll(directory, 0o700); err != nil {
		tb.Fatal(err)
	}
	var manifest strings.Builder
	manifest.WriteString("#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n")
	for segment := range segments {
		name := fmt.Sprintf("segment-%05d.m4s", segment)
		if err := os.WriteFile(filepath.Join(directory, name), []byte("fragment"), 0o600); err != nil {
			tb.Fatal(err)
		}
		fmt.Fprintf(&manifest, "#EXTINF:2.000,\n%s\n", name)
	}
	manifest.WriteString("#EXT-X-ENDLIST\n")
	future := time.Now().Add(time.Second)
	files := map[string]string{"init.mp4": string(mp4fixture.Initialization(640, 360, "h264", "aac", "")), "index.m3u8": manifest.String()}
	for name, content := range files {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			tb.Fatal(err)
		}
		if err := os.Chtimes(path, future, future); err != nil {
			tb.Fatal(err)
		}
	}
	quality := PlaybackQuality{Label: "360p", Width: 640, Height: 360, Bitrate: 493_000}
	master := filepath.Join(filepath.Dir(directory), "index.m3u8")
	if err := WriteMaster(master, "ffmpeg-v1", []PlaybackQuality{quality}, true, osWriteFile); err != nil {
		tb.Fatal(err)
	}
	if err := os.Chtimes(master, future, future); err != nil {
		tb.Fatal(err)
	}
	return master, source
}

func TestCacheFreshNoticesOutputChangedAfterVerification(t *testing.T) {
	for _, change := range []string{"segment-00002.m4s", "init.mp4", "index.m3u8"} {
		t.Run(change, func(t *testing.T) {
			master, source := finishedHLSRendition(t, 4)
			directory := filepath.Join(filepath.Dir(master), "360p")
			for range 2 {
				if !CacheFresh(master, source, "ffmpeg-v1") {
					t.Fatal("finished rendition was not fresh")
				}
			}
			if change == "index.m3u8" {
				writeTestFile(t, filepath.Join(directory, change), "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXT-X-MAP:URI=\"init.mp4\"\n#EXTINF:2.000,\nsegment-00009.m4s\n#EXT-X-ENDLIST\n")
			} else if err := os.Remove(filepath.Join(directory, change)); err != nil {
				t.Fatal(err)
			}
			if CacheFresh(master, source, "ffmpeg-v1") || FinalizedVariant(directory) {
				t.Fatalf("changed %s was still fresh", change)
			}
		})
	}
}

func BenchmarkCacheFreshFinishedRendition(b *testing.B) {
	master, source := finishedHLSRendition(b, 3600)
	b.ReportAllocs()
	for b.Loop() {
		if !CacheFresh(master, source, "ffmpeg-v1") {
			b.Fatal("finished rendition was not fresh")
		}
	}
}
