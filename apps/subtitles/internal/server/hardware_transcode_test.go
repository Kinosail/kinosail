package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
	"github.com/MikeO7/kinosail-subtitles/internal/server"
)

func TestOwnerCanSelectHardwareTranscodingAndToneMapping(t *testing.T) {
	t.Parallel()
	mediaDir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Brazil.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	arguments, toolsDir := filepath.Join(t.TempDir(), "arguments"), t.TempDir()
	ffmpeg, pidFile := filepath.Join(toolsDir, "ffmpeg"), filepath.Join(toolsDir, "pid")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\nprintf '%%s' $$ > '%s'\n", arguments, pidFile) + fakePlayableHLS() + "sleep 30\n"
	//nolint:gosec // G306: the fake FFmpeg adapter must be executable.
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { assertProcessStopsBeforeCleanup(t, pidFile) })
	handler := server.New(server.Config{Lifecycle: t.Context(), MediaDir: mediaDir, CacheDir: cacheDir, FFmpeg: ffmpeg, HardwareOS: "linux", HardwareArch: "amd64"})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=vaapi&toneMap=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/watch/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))
	used, err := os.ReadFile(arguments)
	if err != nil || response.Code != http.StatusSeeOther || playlist.Code != http.StatusOK {
		t.Fatalf("save = %d, playlist = %d, error = %v", response.Code, playlist.Code, err)
	}
	for _, expected := range []string{`<option value="vaapi" selected>AMD or Intel graphics`, `name="toneMap" value="true" checked`} {
		if !strings.Contains(settings.Body.String(), expected) {
			t.Fatalf("settings lacks %q: %q", expected, settings.Body.String())
		}
	}
	for _, expected := range []string{"-hwaccel vaapi", "-c:v h264_vaapi", "tonemap_vaapi"} {
		if !strings.Contains(string(used), expected) {
			t.Fatalf("arguments lack %q: %q", expected, used)
		}
	}
}

func assertProcessStopsBeforeCleanup(t *testing.T, pidFile string) {
	t.Helper()
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		t.Errorf("read fake FFmpeg PID: %v", err)
		return
	}
	pid, err := strconv.Atoi(string(pidBytes))
	if err != nil {
		t.Errorf("parse fake FFmpeg PID: %v", err)
		return
	}
	for range 100 {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Error("fake FFmpeg was still running during test cleanup")
}

func TestTranscoderRejectsUnknownHardware(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir()})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=magic"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("save invalid accelerator = %d %q", response.Code, response.Body.String())
	}
}

func TestUnavailableAndUnknownCodecsAreRejectedWithoutChangingSettings(t *testing.T) {
	t.Parallel()
	tools := t.TempDir()
	ffmpeg := filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, "#!/bin/sh\ncase \"$*\" in *-encoders*) printf 'libx264\\n';; esac\n")
	handler := server.New(server.Config{DataDir: t.TempDir(), FFmpeg: ffmpeg, ProbeHardware: true, HardwareOS: "linux", HardwareArch: "amd64", RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	for _, codec := range []string{"vvc", "av2", "unknown", strings.Repeat("a", 16<<10)} {
		request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&codec="+codec+"&accelerator=auto"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		request.AddCookie(owner)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("web codec %q = %d %q", codec, response.Code, response.Body.String())
		}
	}
	response := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "quality", "codec": "av1", "accelerator": "auto"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unavailable API codec = %d %q", response.Code, response.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"transcoder":"automatic"`, `"codec":"auto"`, `"effectiveCodec":"h264"`)
}

func TestUnsupportedPlatformHardwareIsRejectedWithoutChangingWebOrAPISettings(t *testing.T) {
	t.Parallel()
	handler := server.New(server.Config{DataDir: t.TempDir(), HardwareOS: "linux", HardwareArch: "amd64", RequireAuth: true})
	owner := signInTestProfile(t, handler, "/setup", "name=Owner&password=owner-password")

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=videotoolbox"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("web unsupported accelerator = %d %q", response.Code, response.Body.String())
	}

	api := apiCall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "automatic", "accelerator": "amf"})
	if api.Code != http.StatusBadRequest {
		t.Fatalf("API unsupported accelerator = %d %q", api.Code, api.Body.String())
	}
	assertAPIBody(t, apiCall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"accelerator":"auto"`)
}

func TestUnsupportedPersistedHardwareFallsBackSafely(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), []byte(`{"name":"Kinosail","libraries":["."],"accelerator":"videotoolbox"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: dataDir, HardwareOS: "linux", HardwareArch: "amd64"})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), `<option value="auto" selected>`) || strings.Contains(settings.Body.String(), `<option value="videotoolbox" selected`) {
		t.Fatalf("migrated settings = %d %q", settings.Code, settings.Body.String())
	}
	var persisted struct{ Accelerator string }
	if err := json.Unmarshal(storedState(t, dataDir, "settings.json"), &persisted); err != nil || persisted.Accelerator != "auto" {
		t.Fatalf("persisted accelerator = %q, %v", persisted.Accelerator, err)
	}
}

func TestUnsupportedManagedHardwareFailsClosed(t *testing.T) {
	t.Parallel()
	configured, err := configuration.Load(t.TempDir(), "", func(name string) (string, bool) {
		return "videotoolbox", name == "KINOSAIL_TRANSCODE_ACCELERATOR"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := server.New(server.Config{DataDir: t.TempDir(), HardwareOS: "linux", HardwareArch: "amd64", Configuration: configured})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "videotoolbox") {
		t.Fatalf("unsupported managed hardware = %d %q", response.Code, response.Body.String())
	}
}
