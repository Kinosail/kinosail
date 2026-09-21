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

	"github.com/MikeO7/kinosail-subtitles/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestPlannedHLSDeliversTheTransformationChosenByPlaybackPlanning(t *testing.T) { //nolint:cyclop,funlen,gocognit // Three sources prove the minimum-transform ladder and explicit color/subtitle branch.
	tests := []struct {
		name, probe, query string
		want               []string
		reject             []string
		manifest           []string
		rejectManifest     []string
	}{
		{"remux", `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264","width":1920,"height":804},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1}}],"format":{"format_name":"matroska"}}`, "?compatible=1", []string{"-c:v copy", "-c:a copy", "-hls_time 4", "-hls_flags temp_file+split_by_time"}, []string{"-hls_init_time", "libx264", "independent_segments"}, []string{"RESOLUTION=1920x804", "1080p/index.m3u8"}, []string{"804p/index.m3u8", "#EXT-X-INDEPENDENT-SEGMENTS"}},
		{"audio transcode", `{"streams":[{"index":0,"codec_type":"video","codec_name":"h264"},{"index":1,"codec_type":"audio","codec_name":"truehd","disposition":{"default":1}}],"format":{"format_name":"mp4"}}`, "?compatible=1", []string{"-c:v copy", "-c:a aac", "-ac 2", "-hls_time 4", "-hls_flags temp_file+split_by_time"}, []string{"-hls_init_time", "libx264", "independent_segments"}, nil, []string{"#EXT-X-INDEPENDENT-SEGMENTS"}},
		{"HDR image subtitle", `{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","color_transfer":"smpte2084"},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1}},{"index":3,"codec_type":"subtitle","codec_name":"hdmv_pgs_subtitle"}],"format":{"format_name":"matroska"}}`, "?compatible=1&subtitle=0", []string{"zscale=t=linear", "overlay[base]", "[v0]", "libx264", "-ac 2", "-hls_flags temp_file+independent_segments"}, []string{"-c:v copy", "split_by_time"}, []string{"#EXT-X-INDEPENDENT-SEGMENTS"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
			if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
				t.Fatal(err)
			}
			ffprobe := filepath.Join(tools, "ffprobe")
			writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '"+test.probe+"'\n")
			arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
			output := fakePlayableHLS()
			if test.name == "remux" {
				output = servertest.PlayableHLSWithMedia(1920, 804, "aac")
			}
			writeExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments)+output)
			handler, id := firstWebItem(t, server.Config{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
			page := httptest.NewRecorder()
			handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+test.query, nil))
			match := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())
			if len(match) != 2 || !strings.Contains(match[1], "/p/") {
				t.Fatalf("planned player source = %d %q", page.Code, page.Body.String())
			}
			playlist := httptest.NewRecorder()
			handler.ServeHTTP(playlist, httptest.NewRequestWithContext(t.Context(), http.MethodGet, match[1], nil))
			used, err := os.ReadFile(arguments)
			if playlist.Code != http.StatusOK || err != nil {
				t.Fatalf("planned HLS = %d arguments=%q err=%v", playlist.Code, used, err)
			}
			if !strings.Contains(playlist.Body.String(), ":hls=7") {
				t.Fatalf("planned HLS cache version is stale: %q", playlist.Body.String())
			}
			for _, value := range test.want {
				if !strings.Contains(string(used), value) {
					t.Fatalf("arguments lack %q: %q", value, used)
				}
			}
			for _, value := range test.reject {
				if strings.Contains(string(used), value) {
					t.Fatalf("arguments unexpectedly contain %q: %q", value, used)
				}
			}
			for _, value := range test.manifest {
				if !strings.Contains(playlist.Body.String(), value) {
					t.Fatalf("manifest lacks %q: %q", value, playlist.Body.String())
				}
			}
			for _, value := range test.rejectManifest {
				if strings.Contains(playlist.Body.String(), value) {
					t.Fatalf("manifest unexpectedly contains %q: %q", value, playlist.Body.String())
				}
			}
		})
	}
}
