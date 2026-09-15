package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestHLSRejectsUnavailableAudioBeforeCreatingOutput(t *testing.T) {
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac","index":1}],"format":{"duration":"120"}}'
`)
	handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFprobe: probe, FFmpeg: "must-not-run"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/p/t-a1-s0-none-t0-b0/index.m3u8", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid source recipe = %d %s", response.Code, response.Body.String())
	}
	key := playback.HLSRecipeKey(id, playback.HLSRecipe{Mode: "transcode", Audio: 1})
	if _, err := os.Stat(filepath.Join(cache, key)); !os.IsNotExist(err) {
		t.Fatalf("rejected recipe created output: %v", err)
	}
}

func TestSilentVideoDefaultHLSDoesNotSelectAMissingAudioTrack(t *testing.T) {
	for _, selection := range []string{"", "/audio/0", "/audio/1"} {
		t.Run(selection, func(t *testing.T) {
			assertSilentHLSSelection(t, selection)
		})
	}
}

func assertOnlyProbeCache(t *testing.T, cache string) {
	t.Helper()
	entries, err := os.ReadDir(cache)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "probes" {
			t.Fatalf("rejected audio selection created output: %s", entry.Name())
		}
	}
}

func assertSilentHLSSelection(t *testing.T, selection string) {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Silent.mp4"), []byte("media"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","width":320,"height":180}],"format":{"duration":"120"}}'
`)
	encoder := "must-not-run"
	if selection == "" {
		encoder = filepath.Join(tools, "ffmpeg")
		writeExecutable(t, encoder, "#!/bin/sh\n"+servertest.PlayableHLSWithMedia(320, 180, ""))
	}
	handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFprobe: probe, FFmpeg: encoder})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+selection+"/index.m3u8", nil))
	want := http.StatusOK
	if selection != "" {
		want = http.StatusBadRequest
	}
	if response.Code != want {
		t.Fatalf("playlist status = %d, want %d: %s", response.Code, want, response.Body.String())
	}
	if selection != "" {
		assertOnlyProbeCache(t, cache)
	}
}
