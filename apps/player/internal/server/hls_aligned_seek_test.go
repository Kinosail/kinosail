package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

// These tools preserve the original long-source offset and playlist proof.
// Actual decoder/source-frame identity is owned by the hosted coded fixture.
func alignedSeekFixture(t *testing.T) (http.Handler, string, string, string) {
	t.Helper()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mp4"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(tools, "ffprobe")
	writeExecutable(t, probe, `#!/bin/sh
printf '%s' '{"streams":[{"codec_type":"video","codec_name":"h264","profile":"High","level":40,"width":1920,"height":1080},{"codec_type":"audio","codec_name":"truehd"}],"format":{"format_name":"mp4","duration":"7200"}}'
`)
	arguments, encoder := filepath.Join(tools, "arguments"), filepath.Join(tools, "ffmpeg")
	writeExecutable(t, encoder, fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$*\" >> '%s'\n", arguments)+fakePlayableHLS())
	handler, id := firstWebItem(t, server.Config{Lifecycle: t.Context(), MediaDir: media, CacheDir: cache, FFprobe: probe, FFmpeg: encoder})
	return handler, id, cache, arguments
}

func assertOldAlignedSeekRequiresPlan(t *testing.T, handler http.Handler, id, cache, arguments string) {
	t.Helper()
	page := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/watch/"+id+"?compatible=1", nil))
	match := regexp.MustCompile(`data-hls="([^"]+)"`).FindStringSubmatch(page.Body.String())
	if page.Code != http.StatusOK || len(match) != 2 || !strings.Contains(match[1], "/p/a-") {
		t.Fatalf("original compatible page did not offer audio-compatible copied video: %d", page.Code)
	}
	before := servertest.SnapshotHLSCache(t, cache)
	oldSource := strings.Replace(match[1], "/index.m3u8", "-o2941100/index.m3u8", 1)
	response := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet, oldSource, nil))
	if response.Code != http.StatusConflict {
		t.Fatalf("uncertified old copied seek = %d", response.Code)
	}
	assertNoAlignedSeekEncoder(t, arguments)
	if !reflect.DeepEqual(before, servertest.SnapshotHLSCache(t, cache)) {
		t.Fatal("old copied seek changed HLS cache")
	}
}

func alignedSeekPlaybackSource(t *testing.T, handler http.Handler, id string) string {
	t.Helper()
	response := serveRequest(handler, httptest.NewRequestWithContext(t.Context(), http.MethodGet,
		"/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac&position=2941.1", nil))
	var result struct {
		Compatible      string
		CompatibleLabel string
		CompatiblePlan  struct {
			Allowed bool
			Mode    string
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusOK || !result.CompatiblePlan.Allowed || result.CompatiblePlan.Mode != "transcode" {
		t.Fatalf("aligned position plan = %d, %+v", response.Code, result.CompatiblePlan)
	}
	if !strings.HasPrefix(result.Compatible, "/hls/"+id+"/p/t-") || !strings.HasSuffix(result.Compatible, "/index.m3u8") {
		t.Fatal("aligned plan did not offer the same item's canonical transcode source")
	}
	assertFullOriginSeekConversion(t, result.Compatible, result.CompatibleLabel)
	return result.Compatible
}

func assertNoAlignedSeekEncoder(t *testing.T, arguments string) {
	t.Helper()
	if _, err := os.Stat(arguments); !os.IsNotExist(err) {
		t.Fatalf("seek planning/rejection started an encoder: %v", err)
	}
}
