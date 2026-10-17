package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
)

func TestAppleWebKitReportsNativeSurroundAudio(t *testing.T) {
	t.Parallel()
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
	request.Header.Set("User-Agent", "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 Mobile/15E148 Safari/604.1")
	capabilities := browserPlaybackCapabilitiesForRequest(request, &settingsStore{}, nil)
	if !playback.Includes(capabilities.AudioCodecs, "ac3") || !playback.Includes(capabilities.AudioCodecs, "eac3") {
		t.Fatalf("Apple audio codecs = %#v", capabilities.AudioCodecs)
	}
}
