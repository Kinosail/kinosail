package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/playback"
)

// Real synthetic media and public HTTP expose the same unfinished/stale master
// failure as the observed black intro. This is not Safari or physical-device proof.
func TestRealBlackIntroHLSBandwidthSurvivesCachedMasterReuse(t *testing.T) { //nolint:cyclop,funlen,gocognit // One real encoder lifecycle proves initial, cached, and completed delivery.
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("FFmpeg is required for the HLS bandwidth HTTP integration test")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("FFprobe is required for the HLS bandwidth HTTP integration test")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
	defer cancel()
	media, cache, tools := t.TempDir(), t.TempDir(), t.TempDir()
	fixture := filepath.Join(media, "Black Intro.mkv")
	command := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=24:duration=12,drawbox=color=black:t=fill:enable='lt(t,4)'", "-c:v", "mpeg4", "-q:v", "3", fixture) //nolint:gosec // Fixed generated synthetic fixture and discovered tool.
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generate black-intro fixture: %v: %s", err, output)
	}
	calls := filepath.Join(tools, "calls")
	adapter := filepath.Join(tools, "ffmpeg")
	script := fmt.Sprintf("#!/bin/sh\nprintf 'call\\n' >> '%s'\nexec '%s' -readrate 1 \"$@\"\n", calls, ffmpeg)
	if err := os.WriteFile(adapter, []byte(script), 0o700); err != nil { //nolint:gosec // Executable real-FFmpeg adapter paces only this temporary fixture.
		t.Fatal(err)
	}
	handler, id := formatTestItem(t, server.Config{Lifecycle: ctx, MediaDir: media, CacheDir: cache, FFmpeg: adapter, FFprobe: ffprobe})
	host := httptest.NewServer(handler)
	defer host.Close()
	var info struct {
		Compatible     string
		CompatiblePlan playback.PlaybackPlan
	}
	if err := json.Unmarshal(bandwidthHTTPGET(t, ctx, host.URL+"/api/v1/items/"+id+"/playback?videoCodecs=h264&audioCodecs=aac"), &info); err != nil {
		t.Fatal(err)
	}
	if info.CompatiblePlan.Mode != "transcode" || len(info.CompatiblePlan.Qualities) != 1 {
		t.Fatalf("fixture plan = %+v", info.CompatiblePlan)
	}
	target := info.CompatiblePlan.Qualities[0].Bitrate
	initial := bandwidthHTTPGET(t, ctx, host.URL+info.Compatible)
	rendering := strings.TrimSuffix(info.Compatible, "/index.m3u8")
	variant := speedTestURIs(initial)[0]
	root := filepath.Join(cache, playback.HLSRecipeKey(id, playback.RecipeFor(info.CompatiblePlan)))
	playlist := filepath.Join(root, variant)
	partial, err := os.ReadFile(playlist)
	if err != nil || bytes.Contains(partial, []byte("#EXT-X-ENDLIST")) {
		t.Fatalf("fixture was not unfinished at first publication: %v", err)
	}
	assertBandwidthFloor(t, initial, target)

	// Reproduce an old retained master without altering the real media fragments.
	identity := ""
	for _, line := range strings.Split(string(initial), "\n") {
		if strings.HasPrefix(line, "#KINOSAIL-TRANSCODER:") {
			identity = strings.TrimPrefix(line, "#KINOSAIL-TRANSCODER:")
		}
	}
	if identity == "" {
		t.Fatal("master has no cache identity")
	}
	stale := regexp.MustCompile(`((?:AVERAGE-)?BANDWIDTH=)\d+`).ReplaceAll(initial, []byte("${1}1"))
	stale = bytes.ReplaceAll(stale, []byte(playback.HLSBandwidthPolicyMarker+"\n"), nil)
	if err := os.WriteFile(filepath.Join(root, "index.m3u8"), stale, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".seekable"), []byte(identity), 0o600); err != nil {
		t.Fatal(err)
	}
	segment := filepath.Join(root, strings.TrimSuffix(variant, "index.m3u8"), "segment-00000.m4s")
	before, err := os.Stat(segment)
	if err != nil {
		t.Fatal(err)
	}
	beforeCalls, _ := os.ReadFile(calls)
	for range 3 {
		repaired := bandwidthHTTPGET(t, ctx, host.URL+info.Compatible)
		assertBandwidthFloor(t, repaired, target)
		if !bytes.Equal(stripBandwidth(repaired), stripBandwidth(initial)) {
			t.Error("cached repair changed rendition, codec, resolution, or cache identity")
		}
	}
	afterCalls, _ := os.ReadFile(calls)
	after, err := os.Stat(segment)
	if err != nil || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() || !bytes.Equal(beforeCalls, afterCalls) {
		t.Fatalf("cached repair restarted encoding or changed media: %v", err)
	}

	for {
		partial, err = os.ReadFile(playlist)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(partial, []byte("#EXT-X-ENDLIST")) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("real encoder did not finish")
		case <-time.After(25 * time.Millisecond):
		}
	}
	average, peak := playback.VariantBandwidth(filepath.Dir(playlist), target)
	for {
		final := bandwidthHTTPGET(t, ctx, host.URL+info.Compatible)
		if bytes.Contains(final, fmt.Appendf(nil, "BANDWIDTH=%d,AVERAGE-BANDWIDTH=%d,", peak, average)) {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("completed master does not report measured bitrate: %s", final)
		case <-time.After(25 * time.Millisecond):
		}
	}
	fragments := bandwidthHTTPGET(t, ctx, host.URL+rendering+"/"+strings.TrimSuffix(variant, "index.m3u8")+"init.mp4")
	fragments = append(fragments, bandwidthHTTPGET(t, ctx, host.URL+rendering+"/"+strings.TrimSuffix(variant, "index.m3u8")+"segment-00000.m4s")...)
	decode := exec.CommandContext(ctx, ffmpeg, "-v", "error", "-i", "pipe:0", "-map", "0:v:0", "-f", "null", "-") //nolint:gosec // Real public HTTP bytes from a generated fixture and discovered tool.
	decode.Stdin = bytes.NewReader(fragments)
	if output, err := decode.CombinedOutput(); err != nil {
		t.Fatalf("decode delivered first fragment: %v: %s", err, output)
	}
	t.Logf("synthetic HTTP bandwidth: planned=%d initial=%s completed_average=%d completed_peak=%d; cached repair repeated 3 times without reencoding", target, bandwidthFields(initial), average, peak)
}

func bandwidthHTTPGET(t *testing.T, ctx context.Context, url string) []byte {
	t.Helper()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("public HTTP status=%d read=%v: %s", response.StatusCode, err, data)
	}
	return data
}

func assertBandwidthFloor(t *testing.T, master []byte, target int64) {
	t.Helper()
	fields := regexp.MustCompile(`BANDWIDTH=(\d+),AVERAGE-BANDWIDTH=(\d+)`).FindSubmatch(master)
	if len(fields) != 3 {
		t.Fatalf("master has no bandwidth fields: %s", master)
	}
	peak, _ := strconv.ParseInt(string(fields[1]), 10, 64)
	average, _ := strconv.ParseInt(string(fields[2]), 10, 64)
	if average < target || peak < target*11/10 {
		t.Errorf("unfinished master underestimates black-intro demand: average=%d peak=%d planned=%d", average, peak, target)
	}
}

func stripBandwidth(master []byte) []byte {
	return regexp.MustCompile(`((?:AVERAGE-)?BANDWIDTH=)\d+`).ReplaceAll(master, []byte("${1}BITRATE"))
}

func bandwidthFields(master []byte) string {
	return string(regexp.MustCompile(`BANDWIDTH=\d+,AVERAGE-BANDWIDTH=\d+`).Find(master))
}
