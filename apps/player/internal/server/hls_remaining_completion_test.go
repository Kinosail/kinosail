package server

import (
	"math"
	"os"
	"path/filepath"
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

func TestRemainingInitialAACUninferredCadenceRetainsWindow(t *testing.T) {
	manifest := "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:3,\nsegment-00000.m4s\n#EXTINF:4,\nsegment-00001.m4s\n#EXTINF:5,\nsegment-00002.m4s\n"
	segments, ready := startupWindowSegments([]byte(manifest), 120)
	if !ready || len(segments) != 3 {
		t.Fatalf("available ordinary window rejected before an EOF boundary: ready=%t segments=%d", ready, len(segments))
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

// The public origin journey reproduces mux timing but cannot deterministically
// exercise sub-microsecond playlist rounding, absent assets, or malformed cuts.
func TestRemainingRoundedAACWindowUsesAvailableAssets(t *testing.T) {
	prefix := strings.Split(remainingInitialAACPrefix, "#EXTINF:2.005333,\nsegment-00004.m4s")[0]
	cases := []struct {
		name     string
		manifest string
		duration float64
		missing  string
		ready    bool
	}{
		{"captured-eight-second-window", prefix, 10, "", true},
		{"same-prefix-at-source-eof", prefix, 8, "", false},
		{"unknown-source-duration", prefix, 0, "", false},
		{"source-within-rounding-bound", prefix, 8.000002, "", false},
		{"nonfinite-source-duration", prefix, math.Inf(1), "", false},
		{"missing-init", prefix, 10, "init.mp4", false},
		{"missing-fourth-cut", prefix, 10, "segment-00003.m4s", false},
		{"empty-fourth-cut", prefix, 10, "empty:segment-00003.m4s", false},
		{"greater-than-rounding-shortfall", strings.Replace(prefix, "1.984000", "1.983990", 1), 10, "", false},
		{"374-aac-blocks", strings.Replace(prefix, "1.984000", "1.962667", 1), 10, "", false},
		{"376-aac-blocks-retain-existing-window", strings.Replace(prefix, "1.984000", "2.005333", 1), 10, "", true},
		{"duplicate-cut", strings.Replace(prefix, "segment-00003", "segment-00002", 1), 10, "", false},
		{"missing-cut-number", strings.Replace(prefix, "segment-00003", "segment-00004", 1), 10, "", false},
		{"extra-media-uri", prefix + "segment-00004.m4s\n", 10, "", false},
		{"dangling-duration", prefix + "#EXTINF:2.005333,\n", 10, "", false},
		{"duplicate-duration", strings.Replace(prefix, "#EXTINF:1.984000,", "#EXTINF:1.984000,\n#EXTINF:1.984000,", 1), 10, "", false},
		{"malformed-duration", strings.Replace(prefix, "1.984000", "invalid", 1), 10, "", false},
		{"nonfinite-cut-duration", strings.Replace(prefix, "1.984000", "NaN", 1), 10, "", false},
		{"fifth-physical-cut-cannot-certify-eof", remainingInitialAACPrefix, 10, "", false},
		{"genuine-final-output", remainingInitialAACPrefix + "#EXT-X-ENDLIST\n", 10, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root, directory := remainingStartupAssets(t, tc.manifest, tc.missing)
			ready := startupRenditionReady(root, directory, "generation", "audio/index.m3u8", tc.duration)
			if ready != tc.ready {
				t.Fatalf("asset-backed startup readiness=%t want=%t", ready, tc.ready)
			}
		})
	}
}

func TestRemainingRoundedAACWindowRetainsRejectedProjection(t *testing.T) {
	prefix := strings.Split(remainingInitialAACPrefix, "#EXTINF:2.005333,\nsegment-00004.m4s")[0]
	root, directory := remainingStartupAssets(t, prefix, "")
	if startupRenditionReady(root, directory, "generation", "audio/index.m3u8", 10, func([]byte) []byte { return nil }) {
		t.Fatal("explicitly rejected projection declared rounded startup ready")
	}
}

func remainingStartupAssets(t *testing.T, manifest, missing string) (*os.Root, string) {
	t.Helper()
	cache := t.TempDir()
	directory := filepath.Join(cache, "generation")
	rendition := filepath.Join(directory, "audio")
	if err := os.MkdirAll(rendition, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rendition, "index.m3u8"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{"init.mp4"}, strings.Split(manifest, "\n")...) {
		if _, valid := hlsSegmentNumber(name); !valid && name != "init.mp4" {
			continue
		}
		if name == missing {
			continue
		}
		content := []byte("synthetic asset")
		if "empty:"+name == missing {
			content = nil
		}
		if err := os.WriteFile(filepath.Join(rendition, name), content, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.OpenRoot(cache)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root, directory
}
