package server

import (
	"strings"
	"testing"
)

// The hosted control fails before decode and depends on mux/cleanup timing.
// This captured prefix deterministically protects the readiness decision that
// ends speculative ownership before a genuine final manifest is published.
func TestRemainingInitialAACPrefixCannotCertifyCompletion(t *testing.T) {
	_, ready := startupWindowSegments([]byte(remainingInitialAACPrefix), 10)
	if ready {
		t.Fatal("unfinalized 10.005332-second prefix for a 10-second source declared ready; preparation can stop before ENDLIST")
	}
}

func TestRemainingInitialAACFinalizedOutputRemainsReady(t *testing.T) {
	segments, ready := startupWindowSegments([]byte(remainingInitialAACPrefix+"#EXT-X-ENDLIST\n"), 10)
	if !ready || len(segments) != 5 {
		t.Fatalf("genuine finalized AAC output: ready=%t segments=%d", ready, len(segments))
	}
}

func TestRemainingInitialAACOrdinaryWindowRemainsReady(t *testing.T) {
	segments, ready := startupWindowSegments([]byte(remainingInitialAACPrefix), 120)
	if !ready || len(segments) != 5 {
		t.Fatalf("ordinary bounded preparation window: ready=%t segments=%d", ready, len(segments))
	}
}

func TestRemainingInitialAACRetainsExplicitProjection(t *testing.T) {
	projection := func([]byte) []byte {
		return []byte(strings.ReplaceAll(remainingInitialAACPrefix, "EVENT", "VOD") + "#EXT-X-ENDLIST\n")
	}
	_, ready := startupWindowSegments([]byte(remainingInitialAACPrefix), 10, projection)
	if !ready {
		t.Fatal("the explicit certified projection was replaced by ordinary cadence admission")
	}
}

func TestRemainingInitialAACRejectedProjectionStaysUnready(t *testing.T) {
	_, ready := startupWindowSegments([]byte(remainingInitialAACPrefix), 10, func([]byte) []byte { return nil })
	if ready {
		t.Fatal("an explicitly rejected projection declared startup ready")
	}
}

const remainingInitialAACPrefix = `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:2
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:EVENT
#EXT-X-MAP:URI="init.mp4"
#EXTINF:2.005333,
segment-00000.m4s
#EXTINF:2.005333,
segment-00001.m4s
#EXTINF:2.005333,
segment-00002.m4s
#EXTINF:1.984000,
segment-00003.m4s
#EXTINF:2.005333,
segment-00004.m4s
`
