package downloads

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"

	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

func TestTranscodingPreservesPrimaryAndBuildsSoftwareFallback(t *testing.T) {
	t.Parallel()
	settings := Transcoding(
		func() transcodepolicy.Settings {
			return transcodepolicy.Settings{Codec: "hevc", Accelerator: "vaapi", Encoder: "hevc_vaapi"}
		},
		func(codec string) string { return "libx265-" + codec },
	)
	if primary := settings(false); primary.Accelerator != "vaapi" || primary.Encoder != "hevc_vaapi" {
		t.Fatalf("primary = %#v", primary)
	}
	if fallback := settings(true); fallback.Accelerator != "none" || fallback.Encoder != "libx265-hevc" {
		t.Fatalf("fallback = %#v", fallback)
	}
}

func TestCopyAndIntegrityReportFileFailures(t *testing.T) {
	input := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(input, []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(input, filepath.Join(t.TempDir(), "missing", "output")); err == nil {
		t.Fatal("missing output directory was accepted")
	}
	if err := copyFile(t.TempDir(), filepath.Join(t.TempDir(), "output")); err == nil {
		t.Fatal("directory input was copied")
	}
	if _, _, err := fileIntegrity(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatalf("missing integrity file error = %v", err)
	}
}

func TestVideoDownloadRejectsMissingInspectionBeforeEncoding(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "output.mp4")
	manager := &Manager{ctx: t.Context(), ffmpeg: "must-not-run", transcoding: func(bool) transcodepolicy.Settings {
		return transcodepolicy.Settings{Codec: "h264", Accelerator: "none", Encoder: "libx264"}
	}}
	item := library.Item{Kind: "video", Path: "/library/movie.mkv"}
	if err := manager.encode(item, "720p", output, false); err == nil {
		t.Fatal("missing source facts were accepted")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("rejected source produced output")
	}
	manager.inspect = func(context.Context, library.Item) playback.MediaFacts {
		return playback.MediaFacts{Video: playback.VideoFacts{Codec: "hevc", HDR: "dolby-vision", DolbyVisionCompatibility: 0}}
	}
	if err := manager.encode(item, "720p", output, false); err == nil {
		t.Fatal("unsupported Dolby conversion was accepted")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("rejected Dolby source produced output")
	}
}
