package server_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

func TestHDRSoftwareLadderToneMapsOncePerPresentation(t *testing.T) {
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	ffprobe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, ffprobe, "#!/bin/sh\nprintf '%s' '"+`{"streams":[{"index":0,"codec_type":"video","codec_name":"hevc","width":3840,"height":2160,"r_frame_rate":"24/1","pix_fmt":"yuv420p10le","color_transfer":"smpte2084","field_order":"tt"},{"index":1,"codec_type":"audio","codec_name":"aac","disposition":{"default":1}}],"format":{"format_name":"matroska","bit_rate":"40000000"}}`+"'\n")
	arguments, ffmpeg := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, ffmpeg, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments)+fakePlayableHLS())
	handler, id := firstWebItem(t, server.Config{MediaDir: media, CacheDir: cache, FFprobe: ffprobe, FFmpeg: ffmpeg})
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	match := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(match) != 2 {
		t.Fatalf("player source = %d %q", page.Code, page.Body.String())
	}
	requestContext, cancel := context.WithCancel(t.Context())
	cancel()
	playlist := httptest.NewRecorder()
	handler.ServeHTTP(playlist, httptest.NewRequestWithContext(requestContext, http.MethodGet, match[1], nil))
	used, err := os.ReadFile(arguments)
	if playlist.Code != http.StatusOK || err != nil {
		t.Fatalf("HLS = %d arguments=%q err=%v", playlist.Code, used, err)
	}
	var presentation string
	for line := range strings.SplitSeq(string(used), "\n") {
		if strings.Contains(line, "-hls_segment_filename") && strings.Contains(line, "libx264") {
			presentation = line
		}
	}
	rungs := strings.Count(presentation, "-threads:v ")
	if rungs < 1 || strings.Count(presentation, "tonemap=hable") != 1 || strings.Count(presentation, "bwdif=") != 1 || !strings.Contains(presentation, fmt.Sprintf("split=%d", rungs)) {
		t.Fatalf("tone mapping or deinterlacing repeats per rung: %q", presentation)
	}
	for index := range rungs {
		if !strings.Contains(presentation, fmt.Sprintf("-map [v%d]", index)) {
			t.Fatalf("rung %d is not mapped from the shared graph: %q", index, presentation)
		}
	}
	filterThreads := regexp.MustCompile(`-filter_complex_threads (\d+) `).FindStringSubmatch(presentation)
	encoderThreads := regexp.MustCompile(`-threads:v (\d+) `).FindStringSubmatch(presentation)
	if strings.Contains(presentation, " -vf ") || len(filterThreads) != 2 || len(encoderThreads) != 2 || filterThreads[1] != encoderThreads[1] {
		t.Fatalf("presentation kept per-rung filters or unsized filter threads: %q", presentation)
	}
	if strings.Index(presentation, "scale=w=1920") > strings.Index(presentation, "zscale=t=linear") {
		t.Fatalf("tone mapping runs at source resolution: %q", presentation)
	}
}
