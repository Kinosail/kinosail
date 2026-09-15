package playback

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestDisplayCapabilities(t *testing.T) {
	for _, test := range []struct {
		query    string
		formats  []string
		channels int
	}{
		{"", nil, 0},
		{"hdrFormats=sdr,hdr10,hlg&maxAudioChannels=8", []string{"sdr", "hdr10", "hlg"}, 8},
		{"hdrFormats=sdr&maxAudioChannels=1", []string{"sdr"}, 1},
	} {
		formats, channels, err := RequestedDisplayCapabilities(httptest.NewRequestWithContext(t.Context(), "GET", "/?"+test.query, nil))
		if err != nil || !reflect.DeepEqual(formats, test.formats) || channels != test.channels {
			t.Fatalf("capabilities %q = %v %d %v", test.query, formats, channels, err)
		}
	}
}

func TestDisplayCapabilitiesRejectInvalidHints(t *testing.T) {
	for _, query := range []string{
		"hdrFormats=", "hdrFormats=hdr10", "hdrFormats=sdr,sdr", "hdrFormats=SDR", "hdrFormats=sdr,dolby-vision", "hdrFormats=sdr,unknown",
		"hdrFormats=sdr&hdrFormats=hlg", "hdrFormats=" + strings.Repeat("s", 33), "hdrFormats=sdr,hdr10,hlg,sdr",
		"maxAudioChannels=", "maxAudioChannels=0", "maxAudioChannels=9", "maxAudioChannels=2.0", "maxAudioChannels=02", "maxAudioChannels=+2", "maxAudioChannels=-1", "maxAudioChannels=2&maxAudioChannels=8",
		"hdrFormats=%zz", strings.Repeat("x", 4097),
	} {
		request := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
		request.URL.RawQuery = query
		formats, channels, err := RequestedDisplayCapabilities(request)
		if err == nil || formats != nil || channels != 0 {
			t.Fatalf("accepted %q", query)
		}
	}
	if _, _, err := RequestedDisplayCapabilities(nil); err == nil {
		t.Fatal("accepted missing request")
	}
}
