package servertest

import (
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func (fixture TranscodeFixture) CompatiblePlaybackGeneratesAnAlignedSeekableSuffix(t *testing.T, offset, seconds string) {
	t.Parallel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","profile":"High","level":40,"width":1920,"height":1080},{"codec_type":"audio","codec_name":"truehd"}],"format":{"format_name":"mp4","duration":"7200"}}'
`)
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments)+PlayableHLS())
	handler, id := FirstWebItem(t, fixture.New(TranscodeConfig{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg}))
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	source := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())[1]
	seekSource := strings.Replace(source, "/index.m3u8", "-o"+offset+"/index.m3u8", 1)
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, seekSource, nil))
	used, err := os.ReadFile(arguments)
	if playlist.Code != http.StatusOK || err != nil || !strings.Contains(string(used), "-ss "+seconds) {
		t.Fatalf("seek playlist = %d, arguments = %q, error = %v", playlist.Code, used, err)
	}
}

func (fixture TranscodeFixture) CompatiblePlaybackRejectsInvalidSeekOffsetsWithoutEncoding(t *testing.T) {
	t.Parallel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}],\"format\":{\"format_name\":\"mp4\",\"duration\":\"120\"}}'\n")
	called, ffmpeg := filepath.Join(tools, "called"), filepath.Join(tools, "ffmpeg")
	WriteExecutable(t, ffmpeg, "#!/bin/sh\ntouch '"+called+"'\n"+PlayableHLS())
	handler, id := FirstWebItem(t, fixture.New(TranscodeConfig{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg}))
	before := SnapshotHLSCache(t, cache)
	for _, token := range []string{"o", "oabc", "x30000", "o1", "o101", "o30000-o60000", "o120000", "o604800001"} {
		response := httptest.NewRecorder()
		path := "/hls/" + id + "/p/t-a0-s0-none-t0-b0-" + token + "/index.m3u8"
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound && response.Code != http.StatusBadRequest {
			t.Fatalf("invalid offset %q = %d %q", token, response.Code, response.Body.String())
		}
	}
	if _, err := os.Stat(called); !os.IsNotExist(err) {
		t.Fatalf("invalid seek started FFmpeg: %v", err)
	}
	if !reflect.DeepEqual(before, SnapshotHLSCache(t, cache)) {
		t.Fatal("invalid seek changed cache")
	}
}

func SnapshotHLSCache(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			// Probe metadata is independent of HLS output and may be cached during validation.
			if path == filepath.Join(root, "probes") {
				return filepath.SkipDir
			}
			files[path] = "directory"
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // G122: this walk reads only the isolated, test-owned TempDir cache.
		files[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}
