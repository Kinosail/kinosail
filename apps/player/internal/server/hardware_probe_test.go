package server_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHardwareProbeRunsAfterStartupAndIsReusedAfterRestart(t *testing.T) {
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	device := filepath.Join(tools, "renderD128")
	if err := os.WriteFile(device, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	checks, release, arguments, ffmpeg := filepath.Join(tools, "checks"), filepath.Join(tools, "release"), filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, fmt.Sprintf(`#!/bin/sh
case "$*" in
  *-encoders*) printf ' V..... libx264\n V..... h264_vaapi\n'; exit 0 ;;
  *-hwaccels*) printf 'vaapi\n'; exit 0 ;;
  *kinosail-transcoder-check-*) printf 'check\n' >> '%s'; while [ ! -f '%s' ]; do sleep 0.01; done ;;
  *) printf '%%s\n' "$*" >> '%s' ;;
esac
`, checks, release, arguments)+servertest.HardwareSmokeHLS()+fakePlayableHLS())
	probe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, probe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"mpeg4\",\"width\":1280,\"height\":720},{\"codec_type\":\"audio\",\"codec_name\":\"aac\"}],\"format\":{\"format_name\":\"matroska\",\"duration\":\"60\"}}'\n")
	config := server.Config{Lifecycle: t.Context(), MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFprobe: probe, FFmpeg: ffmpeg, ProbeHardware: true, HardwareDevices: []string{device}, HardwareOS: "linux", HardwareArch: "amd64"}
	started := time.Now()
	handler, id := firstWebItem(t, config)
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("startup waited for hardware checks: %s", elapsed)
	}
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/hardware", nil), http.StatusOK, `"probing":true`, `"status":"Checking hardware"`, `"selected":"none"`)
	assertAPIBody(t, apiCall(t, handler, "", http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "automatic", "codec": "auto", "accelerator": "vaapi"}), http.StatusOK)
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"accelerator":"vaapi"`, `"effectiveAccelerator":"none"`)
	playlist := apiCall(t, handler, "", http.MethodGet, "/hls/"+id+"/index.m3u8", nil)
	used, err := os.ReadFile(arguments)
	if playlist.Code != http.StatusOK || err != nil || !strings.Contains(string(used), "-c:v libx264") || strings.Contains(string(used), "h264_vaapi") {
		t.Fatalf("playback while probing = %d %q %v", playlist.Code, used, err)
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	servertest.AwaitHardwareProbe(t, handler, "")
	assertAPIBody(t, apiCall(t, handler, "", http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"accelerator":"vaapi"`, `"effectiveAccelerator":"vaapi"`)
	firstChecks := countLines(t, checks)
	if firstChecks == 0 {
		t.Fatal("hardware was not verified")
	}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(filepath.Join(cache, "hardware-probe.json")); err == nil {
			break
		}
	}
	if err := os.Remove(release); err != nil {
		t.Fatal(err)
	}
	config.DataDir = t.TempDir()
	restarted, _ := firstWebItem(t, config)
	assertAPIBody(t, apiCall(t, restarted, "", http.MethodGet, "/api/v1/hardware", nil), http.StatusOK, `"selected":"vaapi"`, `"status":"Smoke check passed"`)
	if body := apiCall(t, restarted, "", http.MethodGet, "/api/v1/hardware", nil).Body.String(); strings.Contains(body, `"probing":true`) {
		t.Fatalf("restart repeated hardware probing: %s", body)
	}
	if again := countLines(t, checks); again != firstChecks {
		t.Fatalf("restart ran %d new hardware checks", again-firstChecks)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), "\n")
}
