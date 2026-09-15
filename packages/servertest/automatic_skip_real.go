package servertest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func (fixture AutomaticSkipFixture) RealFFmpegAutomaticSkipCopiesPlayableVideo(t *testing.T) {
	t.Helper()
	ffmpeg, ffprobe := os.Getenv("KINOSAIL_REAL_FFMPEG"), os.Getenv("KINOSAIL_REAL_FFPROBE")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("set KINOSAIL_REAL_FFMPEG and KINOSAIL_REAL_FFPROBE to run the media integration test")
	}
	media := realAutomaticSkipMedia(t, ffmpeg, "30", "10000", "20000")
	handler, token, id, _ := fixture.realAutomaticSkipServer(t, media, ffmpeg, ffprobe)
	playback := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	AssertAPIBody(t, playback, http.StatusOK, `"mode":"audio-transcode"`, `"duration":20`)
	var result struct {
		Compatible string `json:"compatible"`
	}
	MustJSON(t, playback, &result)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	address, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	mediaURL := "http://host.containers.internal:" + address.Port() + result.Compatible
	probe := exec.CommandContext(t.Context(), ffprobe, "-v", "error", "-headers", "Authorization: Bearer "+token+"\r\n", "-show_entries", "stream=codec_type,codec_name,profile:format=duration", "-of", "json", mediaURL) //nolint:gosec // Executable and URL are explicit opt-in test inputs.
	output, err := probe.CombinedOutput()
	if err != nil {
		t.Fatalf("probe shortened HLS: %v: %s", err, output)
	}
	assertRealAutomaticSkipProbe(t, output)
	assertCopiedHLSFrames(t, token, ffmpeg, mediaURL)
	client := &http.Client{Timeout: 10 * time.Second}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, httpServer.URL+result.Compatible, nil) //nolint:gosec // The URL targets the local httptest server created by this test.
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := client.Do(request) //nolint:gosec // The request targets the local httptest server created by this test.
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("shortened HLS status = %d", response.StatusCode)
	}
}

func assertRealAutomaticSkipProbe(t *testing.T, output []byte) {
	t.Helper()
	var decoded struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			CodecName string `json:"codec_name"`
			Profile   string `json:"profile"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatal(err)
	}
	duration, _ := strconv.ParseFloat(decoded.Format.Duration, 64)
	codecs := fmt.Sprint(decoded.Streams)
	if duration < 19.9 || duration > 20.1 || !strings.Contains(codecs, "video h264 Constrained Baseline") || !strings.Contains(codecs, "audio aac") {
		t.Fatalf("shortened HLS duration/codecs = %.3f %s; probe = %s", duration, codecs, output)
	}
}

func (fixture AutomaticSkipFixture) RealFFmpegAutomaticSkipPreservesTranscodedFrames(t *testing.T) {
	t.Helper()
	ffmpeg, ffprobe := os.Getenv("KINOSAIL_REAL_FFMPEG"), os.Getenv("KINOSAIL_REAL_FFPROBE")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("set KINOSAIL_REAL_FFMPEG and KINOSAIL_REAL_FFPROBE to run the media integration test")
	}
	media := realAutomaticSkipMedia(t, ffmpeg, "5", "0", "1000")
	handler, token, id, _ := fixture.realAutomaticSkipServer(t, media, ffmpeg, ffprobe)
	playback := APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil)
	AssertAPIBody(t, playback, http.StatusOK, `"mode":"transcode"`, `"duration":4`)
	var result struct {
		Compatible string `json:"compatible"`
	}
	MustJSON(t, playback, &result)
	httpServer := httptest.NewServer(handler)
	defer httpServer.Close()
	address, err := url.Parse(httpServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	mediaURL := "http://host.containers.internal:" + address.Port() + result.Compatible
	decode := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-headers", "Authorization: Bearer "+token+"\r\n", "-i", mediaURL, "-map", "0:v:0", "-f", "framemd5", "-") //nolint:gosec // Executable and URL are explicit opt-in test inputs.
	output, err := decode.CombinedOutput()
	if err != nil {
		t.Fatalf("decode shortened HLS: %v: %s", err, output)
	}
	frames, hashes := 0, make(map[string]struct{})
	for _, line := range strings.Split(string(output), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, ",")
		frames++
		hashes[strings.TrimSpace(fields[len(fields)-1])] = struct{}{}
	}
	if frames < 94 || len(hashes) < 94 {
		t.Fatalf("shortened HLS frames = %d unique = %d; want at least 94 retained moving frames", frames, len(hashes))
	}
}
