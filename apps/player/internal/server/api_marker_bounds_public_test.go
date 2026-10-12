package server_test

import "testing"

func TestPlaybackMarkerBoundsPublicRealMedia(t *testing.T) {
	playbackAPIContracts.PlaybackMarkerBoundsPublicRealMedia(t, true)
}
