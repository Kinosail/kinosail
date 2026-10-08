package server_test

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"testing"
)

// Reconstruct the historical uncorrected8-second AAC refill on disposable media.
// This changes generated cached fragments, never source media or stored receipts.
func remainingPublicLegacyRefill(t *testing.T, ctx context.Context, ffmpeg, source string) string {
	t.Helper()
	directory := t.TempDir()
	command := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-y", //nolint:gosec // Discovered codec and fixed historical recipe on generated fixture paths.
		"-avoid_negative_ts", "disabled", "-max_delay", "5000000", "-ss", "8.000", "-i", source,
		"-map", "0:a:0", "-vn", "-sn", "-dn", "-c:a", "aac", "-ac", "2", "-b:a", "192000", "-output_ts_offset", "8.000",
		"-f", "hls", "-hls_time", "2", "-hls_playlist_type", "event", "-hls_segment_type", "fmp4",
		"-hls_segment_options", "movflags=+frag_discont+skip_sidx", "-hls_flags", "temp_file", "-hls_fmp4_init_filename", "init.mp4",
		"-start_number", "4", "-hls_segment_filename", filepath.Join(directory, "segment-%05d.m4s"), filepath.Join(directory, "index.m3u8"))
	diagnostic := &remainingPublicBoundedOutput{maximum: 65536}
	command.Stdout, command.Stderr = diagnostic, diagnostic
	if err := command.Run(); err != nil {
		t.Fatalf("historical generated AAC refill failed: %v", err)
	}
	return directory
}

func remainingPublicInstallLegacy(t *testing.T, ctx context.Context, ffmpeg, root, legacy string, reference []byte) map[string]any {
	t.Helper()
	manifest := remainingPublicRead(t, filepath.Join(root, "audio/index.m3u8"))
	names := speedTestURIs(manifest)
	oldNames := speedTestURIs(remainingPublicRead(t, filepath.Join(legacy, "index.m3u8")))
	if len(names) != 6 || len(oldNames) != 2 || oldNames[0] != "segment-00004.m4s" || oldNames[1] != "segment-00005.m4s" {
		t.Fatal("historical generated AAC refill has a different ordinal boundary")
	}
	for _, name := range oldNames {
		remainingPublicWrite(t, filepath.Join(root, "audio", name), remainingPublicRead(t, filepath.Join(legacy, name)))
	}
	joined := remainingPublicRead(t, filepath.Join(root, "audio/init.mp4"))
	for _, name := range names {
		joined = append(joined, remainingPublicRead(t, filepath.Join(root, "audio", name))...)
	}
	oldPCM := remainingPublicPCM(t, ctx, ffmpeg, joined)
	if bytes.Equal(oldPCM, reference) || len(oldPCM) != 480256*4 {
		t.Fatalf("historical AAC collision not reproduced: samples%d equal%t", len(oldPCM)/4, bytes.Equal(oldPCM, reference))
	}
	return map[string]any{
		"historicalRefillStartSeconds": 8, "historicalSamples": len(oldPCM) / 4,
		"historicalPublicSHA256": remainingPublicHash(joined), "historicalPCMSHA256": remainingPublicHash(oldPCM), "differsFromCompleteControl": true,
	}
}
