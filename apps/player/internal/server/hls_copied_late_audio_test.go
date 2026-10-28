package server_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestRemuxedSurroundAudioThatStartsLateHasADecodableHeader(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the late audio integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the late audio integration test")
	}
	for _, codec := range []string{"ac3", "eac3"} {
		t.Run(codec, func(t *testing.T) {
			assertLateSurroundRemux(t, ffmpeg, ffprobe, codec)
		})
	}
}

func assertLateSurroundRemux(t *testing.T, ffmpeg, ffprobe, codec string) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	media := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=60", "-itsoffset", "20", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=40", "-map", "0:v", "-map", "1:a", "-c:v", "libx264", "-preset", "ultrafast", "-g", "48", "-c:a", codec, "-ac", "6", filepath.Join(media, "Film (2020).mkv"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate late audio fixture: %v: %s", err, output)
	}
	cache := t.TempDir()
	handler := server.New(server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	var library struct{ Items []struct{ ID string } }
	copiedSeekJSON(t, handler, "/api/v1/library", &library)
	if len(library.Items) != 1 {
		t.Fatalf("library items = %d", len(library.Items))
	}
	var answer struct {
		Compatible     string
		CompatiblePlan struct{ Mode string }
	}
	copiedSeekJSON(t, handler, "/api/v1/items/"+library.Items[0].ID+"/playback?videoCodecs=h264,hevc&audioCodecs=aac,mp3,ac3,eac3&hdrFormats=sdr&maxAudioChannels=6", &answer)
	if answer.CompatiblePlan.Mode != "remux" {
		t.Fatalf("fixture plan = %q; want remux with surround audio", answer.CompatiblePlan.Mode)
	}
	master := copiedSeekGET(t, handler, answer.Compatible)
	rendition := answer.Compatible[:strings.LastIndex(answer.Compatible, "/")+1] + strings.TrimSpace(lastPlaylistURI(master))
	_, names := playlistSegmentStarts(copiedSeekGET(t, handler, rendition))
	if len(names) < 10 {
		t.Fatalf("remuxed playlist lists %d segments", len(names))
	}
	base := rendition[:strings.LastIndex(rendition, "/")+1]
	initialization := copiedSeekGET(t, handler, base+"init.mp4")
	for _, name := range []string{names[0], names[len(names)-2]} {
		fragment := filepath.Join(t.TempDir(), "fragment.mp4")
		if err := os.WriteFile(fragment, append(append([]byte(nil), initialization...), copiedSeekGET(t, handler, base+name)...), 0o600); err != nil {
			t.Fatal(err)
		}
		output, err := exec.CommandContext(ctx, ffprobe, "-v", "error", "-count_frames", "-select_streams", "a:0", "-show_entries", "stream=codec_name,channels,nb_read_frames", "-of", "csv=p=0", fragment).CombinedOutput()
		fields := strings.Split(strings.Trim(strings.TrimSpace(string(output)), ","), ",")
		if err != nil || len(fields) != 3 || fields[0] != codec || fields[1] != "6" || fields[2] == "0" || fields[2] == "N/A" {
			t.Errorf("%s audio = %q, %v; want decodable 6-channel %s", name, output, err, codec)
		}
	}
}
