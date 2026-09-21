package server

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestRequestedPlaybackCapabilitiesPreserveValidatedDisplayHints(t *testing.T) {
	settings := newSettingsStore(t.TempDir(), "", "", nil)
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/items/item/playback?hdrFormats=sdr,hdr10&maxAudioChannels=6&videoCodecs=h264&audioCodecs=aac", nil)
	client, err := requestedPlaybackCapabilities(request, settings)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(client.HDRFormats, []string{"sdr", "hdr10"}) || client.MaxAudioChannels != 6 {
		t.Fatalf("display capabilities = %#v", client)
	}
	if !reflect.DeepEqual(client.VideoCodecs, []string{"h264"}) || !reflect.DeepEqual(client.AudioCodecs, []string{"aac"}) {
		t.Fatalf("codec capabilities = %#v", client)
	}
}
