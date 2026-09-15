package servertest

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func (fixture AutomaticSkipFixture) realAutomaticSkipServer(t *testing.T, media, ffmpeg, ffprobe string) (http.Handler, string, string, string) {
	t.Helper()
	data, cache := realMediaTempDir(t), realMediaTempDir(t)
	handler := fixture.New(t, AutomaticSkipConfig{Lifecycle: realHLSLifecycle(t, cache), MediaDir: media, DataDir: data, CacheDir: cache, FFmpeg: ffmpeg, FFprobe: ffprobe})
	observeRealHLSEncoder(t)
	setup := APICall(t, handler, "", http.MethodPost, "/api/v1/setup", map[string]any{"name": "Owner", "password": "owner-password", "device": "Real media test", "totp": true})
	var owner struct {
		Token string `json:"token"`
		TOTP  struct {
			Secret string `json:"secret"`
		} `json:"totp"`
	}
	MustJSON(t, setup, &owner)
	AssertAPIBody(t, APICall(t, handler, owner.Token, http.MethodPut, "/api/v1/me/mfa", map[string]any{"code": fixture.TOTP(t, owner.TOTP.Secret, time.Now())}), http.StatusOK, `"enabled":true`)
	var catalog struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	MustJSON(t, APICall(t, handler, owner.Token, http.MethodGet, "/api/v1/library", nil), &catalog)
	if len(catalog.Items) != 1 {
		t.Fatalf("real-media library = %#v", catalog.Items)
	}
	return handler, owner.Token, catalog.Items[0].ID, cache
}

func realAutomaticSkipMedia(t *testing.T, ffmpeg, duration, start, end string) string {
	t.Helper()
	media := realMediaTempDir(t)
	metadata := filepath.Join(media, "chapters.ffmeta")
	if err := os.WriteFile(metadata, []byte(";FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART="+start+"\nEND="+end+"\ntitle=Intro\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(media, "Episode.S01E01.mp4")
	command := exec.CommandContext(t.Context(), ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration="+duration, "-f", "lavfi", "-i", "sine=frequency=1000:sample_rate=48000:duration="+duration, "-f", "ffmetadata", "-i", metadata, "-map", "0:v:0", "-map", "1:a:0", "-map_metadata", "2", "-c:v", "libx264", "-preset", "ultrafast", "-profile:v", "baseline", "-level:v", "3.0", "-pix_fmt", "yuv420p", "-g", "240", "-keyint_min", "240", "-sc_threshold", "0", "-c:a", "aac", "-shortest", path) //nolint:gosec // Executable and paths are explicit opt-in test inputs.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate fixture: %v: %s", err, output)
	}
	return media
}

func realMediaTempDir(t *testing.T) string {
	t.Helper()
	root := os.Getenv("KINOSAIL_REAL_MEDIA_ROOT")
	if root == "" {
		return t.TempDir()
	}
	directory, err := os.MkdirTemp(root, "kinosail-real-media-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) }) //nolint:gosec // Directory was created beneath the explicit test root.
	return directory
}
