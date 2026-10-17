package downloads

import (
	"context"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestEncodingReportsProgressFromFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg is unavailable")
	}
	root := t.TempDir()
	input, output := filepath.Join(root, "input.mp4"), filepath.Join(root, "output.mp4")
	if result, err := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-f", "lavfi", "-i", "color=size=64x64:rate=10", "-t", "3", "-c:v", "libx264", "-pix_fmt", "yuv420p", input).CombinedOutput(); err != nil {
		t.Fatalf("fixture: %v: %s", err, result)
	}
	manager := &Manager{ctx: t.Context(), ffmpeg: ffmpeg, transcoding: func(bool) transcodepolicy.Settings {
		return transcodepolicy.Settings{Preset: "ultrafast", CRF: "30", Codec: "h264", Accelerator: "none", Encoder: "libx264"}
	}, inspect: func(context.Context, library.Item) playback.MediaFacts {
		return playback.MediaFacts{Kind: "video", Container: "mp4", Duration: 3, Video: playback.VideoFacts{Codec: "h264", Width: 64, Height: 64, BitDepth: 8, FrameRate: 10, PixelFormat: "yuv420p"}}
	}}
	var mu sync.Mutex
	var ratios []float64
	ctx := withProgress(t.Context(), func(ratio float64) { mu.Lock(); ratios = append(ratios, ratio); mu.Unlock() })
	if err := manager.encodeSelectedContext(ctx, library.Item{Kind: "video", Path: input}, "720p", output, false, &TrackSelection{Audio: []int{}, Subtitles: []int{}}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(ratios) == 0 || ratios[len(ratios)-1] < 0.9 || ratios[len(ratios)-1] > 1 {
		t.Fatalf("progress ratios = %v, want a final ratio between 0.9 and 1", ratios)
	}
}

func TestProgressWriterIgnoresUnrelatedAndInvalidLines(t *testing.T) {
	var ratios []float64
	writer := &progressWriter{total: 10, report: func(ratio float64) { ratios = append(ratios, ratio) }}
	_, _ = writer.Write([]byte("frame=3\nout_time_us=N/A\nout_time_us=-5\nout_time_us=5000000\nout_time_us=99000000\nprogress=con"))
	if len(ratios) != 2 || ratios[0] != 0.5 || ratios[1] != 1 {
		t.Fatalf("ratios = %v, want [0.5 1]", ratios)
	}
}
