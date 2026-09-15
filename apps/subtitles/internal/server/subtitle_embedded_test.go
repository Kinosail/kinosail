package server_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestSubtitleAppExtractsPreferredEmbeddedTextBeforeProviderSearch(t *testing.T) {
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	video := filepath.Join(media, "Arrival.mp4")
	writeTestFile(t, video, "video")
	ffprobe := filepath.Join(tools, "ffprobe")
	ffmpeg := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":3,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"},"disposition":{"default":1}}],"format":{"format_name":"mp4","duration":"7200"}}'
`)
	writeExecutable(t, ffmpeg, `#!/bin/sh
printf 'WEBVTT\n\n00:00:01.000 --> 00:00:02.000\n<i>Hello</i>\n'
`)
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: ffprobe, FFmpeg: ffmpeg})
	id := firstSubtitleInventoryID(t, handler)

	fetched := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{}`)
	data, err := os.ReadFile(filepath.Join(media, "Arrival.en.srt"))
	if fetched.Code != http.StatusCreated || err != nil || !strings.Contains(string(data), "00:00:01,000 --> 00:00:02,000\n<i>Hello</i>") {
		t.Fatalf("fetch = %d %q, sidecar = %q, error = %v", fetched.Code, fetched.Body.String(), data, err)
	}
}

func TestSubtitleAppAutomaticallyExtractsEmbeddedText(t *testing.T) {
	t.Parallel()
	lifecycle, cancel := context.WithCancel(context.Background())
	defer cancel()
	media, tools, data := t.TempDir(), t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	ffprobe := filepath.Join(tools, "ffprobe")
	ffmpeg := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":3,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"eng"}}]}'
`)
	writeExecutable(t, ffmpeg, `#!/bin/sh
printf '1\n00:00:01,000 --> 00:00:02,000\nAutomatic\n'
`)
	health := `{"version":1,"providers":{"SubDL":{"last_success":` + strconv.FormatInt(time.Now().Unix(), 10) + `}}}`
	writeTestFile(t, filepath.Join(data, "subtitle_provider_health.json"), health)
	_ = server.New(server.Config{Lifecycle: lifecycle, SubtitleApp: true, MediaDir: media, DataDir: data, CacheDir: t.TempDir(), FFprobe: ffprobe, FFmpeg: ffmpeg, BackupDir: t.TempDir(), BackupKey: "test-encryption-key", Subtitles: server.SubtitleConfig{URL: "https://api.subdl.com/api/v1", APIKey: "key"}})
	target := filepath.Join(media, "Arrival.en.srt")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(target); err == nil && strings.Contains(string(data), "Automatic") {
			cancel()
			time.Sleep(50 * time.Millisecond)
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("automatic embedded subtitle was not written")
}

func TestSubtitleAppDoesNotExtractWrongEmbeddedLanguage(t *testing.T) {
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(media, "Arrival.mp4"), "video")
	ffprobe := filepath.Join(tools, "ffprobe")
	ffmpeg := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"index":3,"codec_type":"subtitle","codec_name":"subrip","tags":{"language":"spa"}}]}'
`)
	writeExecutable(t, ffmpeg, "#!/bin/sh\nexit 99\n")
	handler := server.New(server.Config{SubtitleApp: true, MediaDir: media, DataDir: t.TempDir(), CacheDir: t.TempDir(), FFprobe: ffprobe, FFmpeg: ffmpeg})
	id := firstSubtitleInventoryID(t, handler)

	fetched := requestJSON(t, handler, http.MethodPost, "/api/v1/subtitle-library/"+id+"/fetch", `{}`)
	if fetched.Code != http.StatusBadGateway {
		t.Fatalf("wrong language fetch = %d %q", fetched.Code, fetched.Body.String())
	}
	if _, err := os.Stat(filepath.Join(media, "Arrival.en.srt")); !os.IsNotExist(err) {
		t.Fatalf("wrong language created a sidecar: %v", err)
	}
}

func firstSubtitleInventoryID(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := requestApp(t, handler, http.MethodGet, "/api/v1/subtitle-library?view=library", "")
	match := regexp.MustCompile(`"id":"([a-f0-9]+)"`).FindStringSubmatch(response.Body.String())
	if response.Code != http.StatusOK || len(match) != 2 {
		t.Fatalf("subtitle inventory = %d %q", response.Code, response.Body.String())
	}
	return match[1]
}
