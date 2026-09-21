package servertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type HLSSchedulerConfig struct {
	MediaDir, DataDir, CacheDir, FFmpeg, FFprobe, HardwareOS, HardwareArch string
	ProbeHardware                                                          bool
	HardwareDevices                                                        []string
}

type HLSSchedulerFixture struct {
	New         func(HLSSchedulerConfig) http.Handler
	PlayableHLS string
}

// HLSScheduler checks scheduling, complete publication, and hardware recovery through the app handler.
func HLSScheduler(t *testing.T, newHandler func(HLSSchedulerConfig) http.Handler, playableHLS string) {
	t.Helper()
	fixture := HLSSchedulerFixture{New: func(config HLSSchedulerConfig) http.Handler {
		config.FFprobe = HLSSchedulerProbe(t)
		return newHandler(config)
	}, PlayableHLS: playableHLS}
	t.Run("LimitsConcurrentFFmpegWork", fixture.LimitsConcurrentFFmpegWork)
	t.Run("PublishesOneCompleteStableMaster", fixture.PublishesOneCompleteStableMaster)
	t.Run("FallsBackToSoftwareAfterHardwareFailure", fixture.FallsBackToSoftwareAfterHardwareFailure)
}

func (fixture HLSSchedulerFixture) LimitsConcurrentFFmpegWork(t *testing.T) {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	for _, name := range []string{"One.mkv", "Two.mkv"} {
		if err := os.WriteFile(filepath.Join(media, name), []byte("video"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	starts, release, ffmpeg := filepath.Join(tools, "starts"), filepath.Join(tools, "release"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf 'start\\n' >> '%s'\nwhile [ ! -f '%s' ]; do sleep 0.02; done\n", starts, release)+fixture.PlayableHLS)
	handler := fixture.New(HLSSchedulerConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg})
	expected := 1 // One adaptive job reserves all available rendition encoders.
	ids := scheduledWatchIDs(t, handler)
	var wait sync.WaitGroup
	for _, id := range ids {
		wait.Add(1)
		go func(id string) {
			defer wait.Done()
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))
		}(id)
	}
	for deadline := time.Now().Add(5 * time.Second); startedLines(starts) < expected && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
	}
	time.Sleep(100 * time.Millisecond)
	if count := startedLines(starts); count != expected {
		_ = os.WriteFile(release, nil, 0o600)
		wait.Wait()
		t.Fatalf("concurrent FFmpeg work = %d, want %d", count, expected)
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	wait.Wait()
	assertScheduledMasters(t, cache, ids)
}

func uniqueWatchIDs(body string) []string {
	seen, ids := make(map[string]bool), make([]string, 0, 2)
	for _, match := range regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindAllStringSubmatch(body, -1) {
		if !seen[match[1]] {
			seen[match[1]], ids = true, append(ids, match[1])
		}
	}
	return ids
}

func (fixture HLSSchedulerFixture) PublishesOneCompleteStableMaster(t *testing.T) {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	release, ffmpeg := filepath.Join(tools, "release-low"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nwhile [ ! -f '%s' ]; do sleep 0.02; done\n", release)+fixture.PlayableHLS)
	handler, id := FirstWebItem(t, fixture.New(HLSSchedulerConfig{MediaDir: media, CacheDir: cache, FFmpeg: ffmpeg}))
	ready := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))
		ready <- response
	}()
	select {
	case response := <-ready:
		t.Fatalf("master published before the complete presentation: %d %q", response.Code, response.Body.String())
	case <-time.After(150 * time.Millisecond):
	}
	if err := os.WriteFile(release, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-ready:
		if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "540p/index.m3u8") || !strings.Contains(response.Body.String(), "1080p/index.m3u8") {
			t.Fatalf("complete master = %d %q", response.Code, response.Body.String())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("complete master was not published")
	}
}

func (fixture HLSSchedulerFixture) FallsBackToSoftwareAfterHardwareFailure(t *testing.T) {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, "#!/bin/sh\ncase \"$*\" in *-encoders*) printf 'libx264 h264_vaapi\\n'; exit 0;; *-hwaccels*) printf 'vaapi\\n'; exit 0;; esac\n"+HardwareSmokeHLS()+fmt.Sprintf("printf '%%s\\n' \"$*\" >> '%s'\ncase \"$*\" in *h264_vaapi*) printf 'failed to initialise vaapi\\n' >&2; exit 1 ;; esac\n", arguments)+fixture.PlayableHLS)
	device := filepath.Join(tools, "renderD128")
	if err := os.WriteFile(device, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	handler, id := FirstWebItem(t, fixture.New(HLSSchedulerConfig{ProbeHardware: true, HardwareDevices: []string{device}, MediaDir: media, DataDir: t.TempDir(), CacheDir: cache, FFmpeg: ffmpeg, HardwareOS: "linux", HardwareArch: "amd64"}))
	settings := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/settings/transcoder", strings.NewReader("quality=automatic&accelerator=vaapi"))
	settings.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(httptest.NewRecorder(), settings)
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/hls/"+id+"/index.m3u8", nil))
	used, err := os.ReadFile(arguments)
	if err != nil || playlist.Code != http.StatusOK || !strings.Contains(string(used), "h264_vaapi") || !strings.Contains(string(used), "libx264") {
		t.Fatalf("fallback = %d arguments=%q err=%v", playlist.Code, used, err)
	}
}

func startedLines(path string) int {
	data, _ := os.ReadFile(path)
	return strings.Count(string(data), "start\n")
}

func FirstWebItem(t *testing.T, handler http.Handler) (http.Handler, string) {
	t.Helper()
	home := httptest.NewRecorder()
	handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
	match := regexp.MustCompile(`/(?:watch|item)/([a-f0-9]+)`).FindStringSubmatch(home.Body.String())
	if home.Code != http.StatusOK || len(match) != 2 {
		t.Fatal("Library did not expose the media fixture")
	}
	return handler, match[1]
}

// WebItemFactory binds the app constructor for fixtures that accept its configuration.
func WebItemFactory[C any](newHandler func(C) http.Handler) func(*testing.T, C) (http.Handler, string) {
	return func(t *testing.T, config C) (http.Handler, string) {
		t.Helper()
		return FirstWebItem(t, newHandler(config))
	}
}

func assertScheduledMasters(t *testing.T, cache string, ids []string) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		complete := true
		for _, id := range ids {
			master, err := os.ReadFile(filepath.Join(cache, id+"-plan-t-a0-s0-none-t0-b0-z1920x1080", "index.m3u8"))
			if err != nil || !strings.Contains(string(master), "540p/index.m3u8") || !strings.Contains(string(master), "1080p/index.m3u8") {
				complete = false
			}
		}
		if complete {
			time.Sleep(50 * time.Millisecond)
			return
		}
	}
	t.Fatal("admitted FFmpeg work did not finish")
}

func scheduledWatchIDs(t *testing.T, handler http.Handler) []string {
	t.Helper()
	var ids []string
	for deadline := time.Now().Add(5 * time.Second); len(ids) < 2 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		home := httptest.NewRecorder()
		handler.ServeHTTP(home, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		ids = uniqueWatchIDs(home.Body.String())
	}
	if len(ids) != 2 {
		t.Fatalf("scanned media items = %d, want 2", len(ids))
	}
	return ids
}

func HLSSchedulerProbe(t *testing.T) string {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "ffprobe")
	WriteExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"hevc","width":1920,"height":1080},{"codec_type":"audio","codec_name":"aac","index":1}],"format":{"duration":"120"}}'
`)
	return probe
}
