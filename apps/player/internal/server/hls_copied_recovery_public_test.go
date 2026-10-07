package server_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// Hosted codecs and real authenticated application requests protect segment0 refill.
func TestCopiedRecoveryRealFirstFragmentRegenerationReopens(t *testing.T) {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		t.Skip("Real media/build proof runs on the hosted runner; focused local tests use controlled stand-ins")
	}
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Fatal("hosted FFmpeg unavailable")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Fatal("hosted FFprobe unavailable")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	media := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-nostdin", "-v", "error", "-f", "lavfi", "-i", "testsrc2=s=320x180:r=24:d=12", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=48000:duration=12", "-c:v", "libx264", "-threads", "2", "-preset", "veryfast", "-g", "48", "-keyint_min", "48", "-sc_threshold", "0", "-c:a", "aac", "-avoid_negative_ts", "disabled", filepath.Join(media, "Episode.S01E01.mp4")) //nolint:gosec // Fixed bounded synthetic input and discovered hosted codec.
	if err := command.Run(); err != nil {
		t.Fatal("bounded source fixture failed")
	}
	cache := t.TempDir()
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	var info struct{ Compatible string }
	mustJSON(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac", nil), &info)
	for {
		var preparation struct{ State string }
		mustJSON(t, apiCall(t, handler, "", http.MethodPost, "/api/v1/items/"+id+"/playback-prepare", map[string]any{"source": info.Compatible}), &preparation)
		if preparation.State == "ready" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("prepared copied cache not ready")
		case <-time.After(20 * time.Millisecond):
		}
	}
	master := speedTestGET(t, ctx, handler, info.Compatible)
	variants := speedTestURIs(master)
	if len(variants) != 1 {
		t.Fatal("fixture needs one copied rendition")
	}
	variantURL := info.Compatible[:len(info.Compatible)-len("index.m3u8")] + variants[0]
	base := variantURL[:len(variantURL)-len("index.m3u8")]
	beforePlaylist := speedTestGET(t, ctx, handler, variantURL)
	for _, name := range speedTestURIs(beforePlaylist) {
		speedTestGET(t, ctx, handler, base+name)
	}
	beforeInit := speedTestGET(t, ctx, handler, base+"init.mp4")
	beforeFirst := speedTestGET(t, ctx, handler, base+"segment-00000.m4s")
	roots, err := filepath.Glob(filepath.Join(cache, id+"-plan-*"))
	if err != nil || len(roots) != 1 {
		t.Fatal("exact copied recipe cache missing")
	}
	if _, err := os.Stat(filepath.Join(roots[0], ".copy-timeline")); err != nil {
		t.Fatal("public fixture did not select indexed copied preparation")
	}
	firstPath := filepath.Join(roots[0], filepath.Dir(variants[0]), "segment-00000.m4s")
	remaining := filepath.Join(filepath.Dir(firstPath), "segment-00001.m4s")
	retained, err := os.Stat(remaining)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(firstPath); err != nil {
		t.Fatal(err)
	}
	regenerated := speedTestGET(t, ctx, handler, base+"segment-00000.m4s")
	afterPlaylist := speedTestGET(t, ctx, handler, variantURL)
	afterInit := speedTestGET(t, ctx, handler, base+"init.mp4")
	afterRemaining, err := os.Stat(remaining)
	if err != nil || !os.SameFile(retained, afterRemaining) {
		t.Fatal("refill/reopen discarded an existing indexed fragment")
	}
	if !bytes.Equal(beforePlaylist, afterPlaylist) || !bytes.Equal(beforeInit, afterInit) || !bytes.Equal(beforeFirst, regenerated) {
		t.Fatal("segment0 refill changed committed metadata/init/media")
	}
	t.Logf("real copied segment0 delete/regenerate/reopen: playlist=%x init=%x first=%x existing-fragment-retained=true", sha256.Sum256(afterPlaylist), sha256.Sum256(afterInit), sha256.Sum256(regenerated))
}
