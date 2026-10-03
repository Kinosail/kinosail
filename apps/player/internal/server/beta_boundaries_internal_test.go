package server

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/dlna"
	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSecurityClassificationAndMediaLimits(t *testing.T) {
	servertest.SecurityClassificationAndMediaLimits(t, collectionMatches, within)
}

func TestDLNADiscoveryBoundaries(t *testing.T) {
	t.Parallel()
	response := dlna.SSDPResponse("http://media.test/", "token")
	if !strings.Contains(response, "LOCATION: http://media.test/dlna/token/device.xml") || !strings.Contains(response, "ST: urn:schemas-upnp-org:device:MediaServer:1") || strings.Contains(response, "token?") {
		t.Fatalf("SSDP response = %q", response)
	}
	for kind, want := range map[string]string{"audio": "object.item.audioItem.musicTrack", "photo": "object.item.imageItem.photo", "video": "object.item.videoItem.movie"} {
		if got := dlna.Class(library.Item{Kind: kind}); got != want {
			t.Fatalf("dlnaClass(%q) = %q, want %q", kind, got, want)
		}
	}
}
