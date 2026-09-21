package servertest

import (
	"context"
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
)

// HardwareTranscodeConfig carries the app-specific server setup for hardware contracts.
type HardwareTranscodeConfig[S any] struct {
	Lifecycle                                                              context.Context
	MediaDir, DataDir, CacheDir, FFprobe, FFmpeg, HardwareOS, HardwareArch string
	RequireAuth, ProbeHardware                                             bool
	Configuration                                                          S
	HardwareDevices                                                        []string
}

// HardwareTranscodeFixture verifies selection, rejection, migration, and process cleanup.
type HardwareTranscodeFixture[S any] struct {
	New         func(HardwareTranscodeConfig[S]) http.Handler
	Load        func(string, string, func(string) (string, bool)) (S, error)
	SignIn      func(*testing.T, http.Handler, string, string) *http.Cookie
	StoredState func(*testing.T, string, string) []byte
}

func (fixture HardwareTranscodeFixture[S]) OwnerCanSelectHardwareTranscodingAndToneMapping(t *testing.T) {
	t.Parallel()
	mediaDir, cacheDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(mediaDir, "Brazil.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	arguments, toolsDir := filepath.Join(t.TempDir(), "arguments"), t.TempDir()
	ffmpeg, pidFile := filepath.Join(toolsDir, "ffmpeg"), filepath.Join(toolsDir, "pid")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\nprintf '%%s' $$ > '%s'\n", arguments, pidFile) + PlayableHLS() + "sleep 30\n"
	//nolint:gosec // G306: the fake FFmpeg adapter must be executable.
	if err := os.WriteFile(ffmpeg, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { assertProcessStopsBeforeCleanup(t, pidFile) })
	probe := filepath.Join(toolsDir, "ffprobe")
	WriteExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":1080,"color_transfer":"smpte2084"},{"codec_type":"audio","codec_name":"aac"}]}'
`)
	handler := fixture.New(HardwareTranscodeConfig[S]{FFprobe: probe, Lifecycle: t.Context(), MediaDir: mediaDir, CacheDir: cacheDir, FFmpeg: ffmpeg, HardwareOS: "linux", HardwareArch: "amd64"})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=vaapi&toneMap=true"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	id := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())[1]
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
	for _, expected := range []string{"-c:v h264_vaapi", "zscale=t=linear", "tonemap=hable", "hwupload"} {
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

func (fixture HardwareTranscodeFixture[S]) TranscoderRejectsUnknownHardware(t *testing.T) {
	t.Parallel()
	handler := fixture.New(HardwareTranscodeConfig[S]{DataDir: t.TempDir()})
	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=magic"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("save invalid accelerator = %d %q", response.Code, response.Body.String())
	}
}

func (fixture HardwareTranscodeFixture[S]) UnavailableAndUnknownCodecsAreRejectedWithoutChangingSettings(t *testing.T) {
	t.Parallel()
	tools := t.TempDir()
	ffmpeg := filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, "#!/bin/sh\ncase \"$*\" in *-encoders*) printf 'libx264\\n'; exit 0;; esac\n"+HardwareSmokeHLS())
	handler := fixture.New(HardwareTranscodeConfig[S]{DataDir: t.TempDir(), FFmpeg: ffmpeg, ProbeHardware: true, HardwareOS: "linux", HardwareArch: "amd64", RequireAuth: true})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")

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
	response := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "quality", "codec": "av1", "accelerator": "auto"})
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unavailable API codec = %d %q", response.Code, response.Body.String())
	}
	AssertAPIBody(t, APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"transcoder":"automatic"`, `"codec":"auto"`, `"effectiveCodec":"h264"`)
}

func (fixture HardwareTranscodeFixture[S]) UnsupportedPlatformHardwareIsRejectedWithoutChangingWebOrAPISettings(t *testing.T) {
	t.Parallel()
	handler := fixture.New(HardwareTranscodeConfig[S]{DataDir: t.TempDir(), HardwareOS: "linux", HardwareArch: "amd64", RequireAuth: true})
	owner := fixture.SignIn(t, handler, "/setup", "name=Owner&password=owner-password")

	request := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=videotoolbox"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(owner)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("web unsupported accelerator = %d %q", response.Code, response.Body.String())
	}

	api := APICall(t, handler, owner.Value, http.MethodPut, "/api/v1/settings/transcoder", map[string]any{"quality": "automatic", "accelerator": "amf"})
	if api.Code != http.StatusBadRequest {
		t.Fatalf("API unsupported accelerator = %d %q", api.Code, api.Body.String())
	}
	AssertAPIBody(t, APICall(t, handler, owner.Value, http.MethodGet, "/api/v1/settings", nil), http.StatusOK, `"accelerator":"auto"`)
}

func (fixture HardwareTranscodeFixture[S]) UnsupportedPersistedHardwareFallsBackSafely(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dataDir, "settings.json"), []byte(`{"name":"Kinosail","libraries":["."],"accelerator":"videotoolbox"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(HardwareTranscodeConfig[S]{DataDir: dataDir, HardwareOS: "linux", HardwareArch: "amd64"})
	settings := httptest.NewRecorder()
	handler.ServeHTTP(settings, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if settings.Code != http.StatusOK || !strings.Contains(settings.Body.String(), `<option value="auto" selected>`) || strings.Contains(settings.Body.String(), `<option value="videotoolbox" selected`) {
		t.Fatalf("migrated settings = %d %q", settings.Code, settings.Body.String())
	}
	var persisted struct{ Accelerator string }
	if err := json.Unmarshal(fixture.StoredState(t, dataDir, "settings.json"), &persisted); err != nil || persisted.Accelerator != "auto" {
		t.Fatalf("persisted accelerator = %q, %v", persisted.Accelerator, err)
	}
}

func (fixture HardwareTranscodeFixture[S]) UnsupportedManagedHardwareFailsClosed(t *testing.T) {
	t.Parallel()
	configured, err := fixture.Load(t.TempDir(), "", func(name string) (string, bool) {
		return "videotoolbox", name == "KINOSAIL_TRANSCODE_ACCELERATOR"
	})
	if err != nil {
		t.Fatal(err)
	}
	handler := fixture.New(HardwareTranscodeConfig[S]{DataDir: t.TempDir(), HardwareOS: "linux", HardwareArch: "amd64", Configuration: configured})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/settings", nil))
	if response.Code != http.StatusServiceUnavailable || strings.Contains(response.Body.String(), "videotoolbox") {
		t.Fatalf("unsupported managed hardware = %d %q", response.Code, response.Body.String())
	}
}

func (fixture HardwareTranscodeFixture[S]) Run(t *testing.T) {
	t.Run("OwnerCanSelectHardwareTranscodingAndToneMapping", fixture.OwnerCanSelectHardwareTranscodingAndToneMapping)
	t.Run("TranscoderRejectsUnknownHardware", fixture.TranscoderRejectsUnknownHardware)
	t.Run("UnavailableAndUnknownCodecsAreRejectedWithoutChangingSettings", fixture.UnavailableAndUnknownCodecsAreRejectedWithoutChangingSettings)
	t.Run("UnsupportedPlatformHardwareIsRejectedWithoutChangingWebOrAPISettings", fixture.UnsupportedPlatformHardwareIsRejectedWithoutChangingWebOrAPISettings)
	t.Run("UnsupportedPersistedHardwareFallsBackSafely", fixture.UnsupportedPersistedHardwareFallsBackSafely)
	t.Run("UnsupportedManagedHardwareFailsClosed", fixture.UnsupportedManagedHardwareFailsClosed)
}

// HardwareTranscodeServer maps the fixture's explicit fields to either app Config.
// Missing or incompatible fields fail immediately instead of silently omitting setup.
func HardwareTranscodeServer[C, S any](newHandler func(C) http.Handler) func(HardwareTranscodeConfig[S]) http.Handler {
	return ServerConfigAdapter[C, HardwareTranscodeConfig[S]](newHandler)
}
