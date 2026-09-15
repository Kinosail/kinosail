package servertest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// PlaybackAPIRejectsInvalidCodecEvidenceBeforeProbingMedia checks rejected input causes no media probe.
func (fixture PlaybackAPIFixture) PlaybackAPIRejectsInvalidCodecEvidenceBeforeProbingMedia(t *testing.T) {
	t.Helper()
	t.Parallel()
	media, tools := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(media, "Film.mkv"), []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	calls, ffprobe := filepath.Join(tools, "calls"), filepath.Join(tools, "ffprobe")
	WriteExecutable(t, ffprobe, "#!/bin/sh\nprintf x >> '"+calls+"'\nprintf '%s' '{\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\"}]}'\n")
	handler, id := FirstWebItem(t, fixture.New(PlaybackAPIConfig{MediaDir: media, FFprobe: ffprobe}))
	_ = os.Remove(calls)
	for _, query := range []string{"videoCodecs=", "videoCodecs=h264%2Ch264", "videoCodecs=unknown", "videoCodecs=h264&videoCodecs=av1", "videoCodecs=" + strings.Repeat("a", 65), "videoCodecs=h264%2Chevc%2Cav1%2Cvp9%2Ch264"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/api/v1/items/%s/playback?%s", id, query), nil))
		if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "video codecs are invalid") {
			t.Fatalf("%s = %d %q", query, response.Code, response.Body.String())
		}
		if _, err := os.Stat(calls); !os.IsNotExist(err) {
			t.Fatalf("%s probed media before validation: %v", query, err)
		}
	}
	for _, query := range []string{"hdrFormats=", "hdrFormats=hdr10", "hdrFormats=sdr,unknown", "hdrFormats=sdr,sdr", "hdrFormats=sdr&hdrFormats=hlg", "hdrFormats=" + strings.Repeat("x", 33), "maxAudioChannels=0", "maxAudioChannels=9", "maxAudioChannels=2.0", "maxAudioChannels=2&maxAudioChannels=8"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("/api/v1/items/%s/playback?%s", id, query), nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid display hint %q = %d", query, response.Code)
		}
		if _, err := os.Stat(calls); !os.IsNotExist(err) {
			t.Fatalf("invalid display hint probed media: %v", err)
		}
	}
}
