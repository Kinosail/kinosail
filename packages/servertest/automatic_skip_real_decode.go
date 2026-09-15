package servertest

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func (fixture AutomaticSkipFixture) RealFFmpegAutomaticSkipServesFutureSegments(t *testing.T) {
	t.Helper()
	ffmpeg, ffprobe := os.Getenv("KINOSAIL_REAL_FFMPEG"), os.Getenv("KINOSAIL_REAL_FFPROBE")
	if ffmpeg == "" || ffprobe == "" {
		t.Skip("set KINOSAIL_REAL_FFMPEG and KINOSAIL_REAL_FFPROBE to run the media integration test")
	}
	media := realAutomaticSkipMedia(t, ffmpeg, "30", "10000", "20000")
	handler, token, id, cache := fixture.realAutomaticSkipServer(t, media, ffmpeg, ffprobe)
	var playback struct {
		Compatible string `json:"compatible"`
	}
	MustJSON(t, APICall(t, handler, token, http.MethodGet, "/api/v1/items/"+id+"/playback", nil), &playback)
	master := APICall(t, handler, token, http.MethodGet, playback.Compatible, nil)
	variantURL := copiedHLSReference(t, playback.Compatible, master.Body.String(), false)
	variant := APICall(t, handler, token, http.MethodGet, variantURL, nil)
	AssertAPIBody(t, variant, http.StatusOK, "#EXT-X-PLAYLIST-TYPE:VOD", "#EXT-X-ENDLIST")
	cached := assertCopiedHLSFutureAbsent(t, cache, variantURL, variant.Body.String())
	assertCopiedHLSSegment(t, handler, token, copiedHLSReference(t, variantURL, variant.Body.String(), true))
	for _, line := range strings.Split(variant.Body.String(), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			assertCopiedHLSSegment(t, handler, token, copiedHLSReference(t, variantURL, line, false))
		}
	}
	assertCopiedHLSFinalCount(t, cached, variant.Body.String(), fixture.CopiedHLSSegments)
}

func assertCopiedHLSSegment(t *testing.T, handler http.Handler, token, path string) {
	t.Helper()
	response := APICall(t, handler, token, http.MethodGet, path, nil)
	if response.Code != http.StatusOK || response.Body.Len() == 0 {
		t.Fatalf("advertised segment returned %d with %d bytes", response.Code, response.Body.Len())
	}
}

func assertCopiedHLSFrames(t *testing.T, token, ffmpeg, mediaURL string) {
	t.Helper()
	//nolint:gosec // The executable and media URL are created by this bounded test fixture.
	decode := exec.CommandContext(t.Context(), ffmpeg, "-v", "error", "-xerror", "-headers", "Authorization: Bearer "+token+"\r\n", "-i", mediaURL, "-map", "0:v:0", "-f", "framemd5", "-")
	output, err := decode.CombinedOutput()
	if err != nil {
		t.Fatalf("copied-video decode: %v: %s", err, output)
	}
	frames := 0
	for _, line := range strings.Split(string(output), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			frames++
		}
	}
	if frames != 480 {
		t.Fatalf("copied-video frames = %d; want 480 retained frames", frames)
	}
}

func copiedHLSReference(t *testing.T, base, manifest string, last bool) string {
	t.Helper()
	var reference string
	for _, line := range strings.Split(manifest, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		reference = line
		if !last {
			break
		}
	}
	address, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(reference)
	if err != nil {
		t.Fatal(err)
	}
	if reference == "" {
		t.Fatal("playlist contains no media reference")
	}
	return address.ResolveReference(parsed).String()
}
