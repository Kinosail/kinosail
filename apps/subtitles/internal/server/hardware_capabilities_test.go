package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestHardwareCapabilitiesDriveAutomaticSelectionAndVisibleFallback(t *testing.T) { //nolint:cyclop,funlen // One end-to-end scenario proves selection, delivery, and diagnostics together.
	t.Parallel()

	media, data, cache, tools := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	device := filepath.Join(tools, "renderD128")
	if err := os.WriteFile(device, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	script := fmt.Sprintf(`#!/bin/sh
case "$*" in
  *-encoders*) printf ' V..... libx264\n V..... libx265\n V..... libsvtav1\n V..... libvpx-vp9\n V..... h264_qsv\n V..... hevc_qsv\n V..... h264_vaapi\n V..... h264_nvenc\n' ;;
  *-hwaccels*) printf 'qsv\nvaapi\ncuda\n' ;;
  *) printf '%%s\n' "$*" >> '%s'; %s ;;
esac
`, arguments, fakePlayableHLS())
	writeExecutable(t, ffmpeg, script)
	handler := server.New(server.Config{MediaDir: media, DataDir: data, CacheDir: cache, FFmpeg: ffmpeg, ProbeHardware: true, HardwareDevices: []string{device}, HardwareOS: "linux", HardwareArch: "amd64", RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")
	var session struct {
		Token string `json:"token"`
	}
	session.Token = owner.Value
	hardware := apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/hardware", nil)
	assertAPIBody(t, hardware, http.StatusOK, `"selected":"qsv"`, "Intel Quick Sync", `"supported":true`, `"usable":true`, `"status":"Ready"`, "NVIDIA NVENC/NVDEC", "Container Toolkit", `"supported":false`, "native macOS Server", "native Windows Server", "Linux V4L2 hardware encoder", "Windows Media Foundation")
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&codec=hevc&accelerator=auto"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	assertAPIBody(t, apiCall(t, handler, session.Token, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"codec":"hevc"`, `"effectiveCodec":"hevc"`, `"accelerator":"auto"`, `"effectiveAccelerator":"qsv"`)
	home := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(home, request)
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(httptest.NewRecorder(), request)
	variantsReady := false
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		master, _ := os.ReadFile(filepath.Join(cache, id, "index.m3u8"))
		if strings.Contains(string(master), "360p/index.m3u8") && strings.Contains(string(master), "1080p/index.m3u8") {
			time.Sleep(50 * time.Millisecond)
			variantsReady = true
			break
		}
	}
	if !variantsReady {
		t.Fatal("adaptive hardware variants did not finish")
	}
	used, err := os.ReadFile(arguments)
	if err != nil || !strings.Contains(string(used), "-hwaccel qsv") || !strings.Contains(string(used), "-c:v hevc_qsv") || !strings.Contains(masterCodec(t, cache, id), `CODECS="hvc1"`) {
		t.Fatalf("automatic hardware arguments = %q, %v", used, err)
	}
	settings := httptest.NewRecorder()
	request = httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil)
	request.AddCookie(owner)
	handler.ServeHTTP(settings, request)
	assertAPIBody(t, settings, http.StatusOK, `<option value="hevc" selected >HEVC / H.265`, "Video conversion", "Automatic will use:</strong> Intel graphics", "Video compatibility", "Ways to convert video", "VVC / H.266", "Not ready yet", "AV2", "Recommended", "NVIDIA graphics", "Needs setup", "Technical details", "NVIDIA NVENC/NVDEC", "Next step:")
	if strings.Contains(settings.Body.String(), "Apple VideoToolbox") || strings.Contains(settings.Body.String(), `value="videotoolbox"`) {
		t.Fatal("settings show hardware for a different operating system")
	}
}

func masterCodec(t *testing.T, cache, id string) string {
	t.Helper()
	master, err := os.ReadFile(filepath.Join(cache, id, "index.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	return string(master)
}
