package downloads

import (
	"context"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestCompatibleDownloadCopiesVideoWithoutEncoderOptions(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is unavailable")
	}
	root := t.TempDir()
	input, output := filepath.Join(root, "input.mp4"), filepath.Join(root, "output.mp4")
	if result, err := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=size=64x64:rate=1", "-t", "1", "-c:v", "libx264", "-pix_fmt", "yuv420p", input).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, result)
	}
	manager := &Manager{ctx: t.Context(), ffmpeg: ffmpeg, transcoding: func(bool) transcodepolicy.Settings {
		return transcodepolicy.Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264"}
	}, inspect: func(context.Context, library.Item) playback.MediaFacts {
		return playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 1, Video: playback.VideoFacts{Codec: "h264", Width: 64, Height: 64, BitDepth: 8, FrameRate: 1, PixelFormat: "yuv420p"}}
	}}
	if err := manager.encodeSelected(library.Item{Kind: "video", Path: input}, "compatible", output, false, &TrackSelection{Audio: []int{}, Subtitles: []int{}}); err != nil {
		t.Fatal(err)
	}
	if result, err := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-i", output, "-f", "null", "-").CombinedOutput(); err != nil {
		t.Fatalf("output decode: %v: %s", err, result)
	}
}
