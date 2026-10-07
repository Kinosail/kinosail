package server_test

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
)

// FFprobe 6 omits duration_time for the final fMP4 AAC packet. The decoder's
// actual LC sample count still proves its finite duration; no source-duration
// estimate may authorize the advertised terminal fragment.
func TestRealAudioHLSHTTPAdmitsDecodedTerminalFrameWithoutPacketDuration(t *testing.T) {
	f, _ := realAudioHLSFixture(t, ".flac", "flac")
	metadata := `{"streams":[{"codec_name":"aac","profile":"LC","sample_rate":"48000"}],"frames":[{"pts_time":"10.005333","nb_samples":1024}]}`
	if err := os.WriteFile(f.endpoint, []byte(metadata), 0o600); err != nil {
		t.Fatal(err)
	}
	before := snapshotCopiedPolicyCache(t, f, true)
	path := strings.TrimSuffix(f.source, "index.m3u8") + "audio/segment-00005.m4s"
	response := apiCall(t, server.New(f.config), "", http.MethodGet, path, nil)
	assertAPIBody(t, response, http.StatusOK)
	actual, err := os.ReadFile(filepath.Join(f.directory, "audio", "segment-00005.m4s"))
	if err != nil || !bytes.Equal(actual, response.Body.Bytes()) {
		t.Fatal("decoded terminal metadata did not deliver its existing media")
	}
	if !bytes.Equal(before, snapshotCopiedPolicyCache(t, f, true)) {
		t.Fatal("decoded terminal metadata rebuilt or mutated its cache")
	}
}
