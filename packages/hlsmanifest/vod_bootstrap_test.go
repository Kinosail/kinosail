package hlsmanifest

import (
	"math"
	"strings"
	"testing"
)

func TestCompleteVODDoesNotExtrapolateBootstrapFragments(t *testing.T) {
	const bootstrap = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:0.041667,\nsegment-00000.m4s\n#EXTINF:0.041667,\nsegment-00001.m4s\n"
	for name, manifest := range map[string]string{
		"initial copied-video fragments": bootstrap,
		"transition to steady cadence":   bootstrap + "#EXTINF:1.916667,\nsegment-00002.m4s\n",
	} {
		t.Run(name, func(t *testing.T) {
			result, ready := CompleteVOD([]byte(manifest), 20, 4)
			if got := string(result); ready || got != manifest {
				t.Fatalf("variable startup fragments extrapolated into %d segments", strings.Count(got, "#EXTINF:"))
			}
		})
	}
}

func TestCompleteVODPreservesCapturedBootstrapOnceCadenceIsEstablished(t *testing.T) {
	const prefix = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:0.041667,\nsegment-00000.m4s\n#EXTINF:0.041667,\nsegment-00001.m4s\n#EXTINF:1.916667,\nsegment-00002.m4s\n#EXTINF:4.000000,\nsegment-00003.m4s\n"
	result, ready := CompleteVOD([]byte(prefix), 20, 4)
	want := strings.Replace(prefix, ":EVENT", ":VOD", 1) + "#EXTINF:4.000000,\nsegment-00004.m4s\n#EXTINF:4.000000,\nsegment-00005.m4s\n#EXTINF:4.000000,\nsegment-00006.m4s\n#EXTINF:1.999999,\nsegment-00007.m4s\n#EXT-X-ENDLIST\n"
	if !ready || string(result) != want {
		t.Fatalf("captured projection = %q, ready=%t", result, ready)
	}
}

func TestCompleteVODRejectsInvalidConfiguredCadence(t *testing.T) {
	const manifest = "#EXTM3U\n#EXT-X-PLAYLIST-TYPE:EVENT\n#EXTINF:2,\nsegment-00000.m4s\n"
	for _, cadence := range []float64{0, -1, 61, math.NaN(), math.Inf(1), math.Inf(-1)} {
		result, ready := CompleteVOD([]byte(manifest), 20, cadence)
		if ready || string(result) != manifest {
			t.Fatalf("invalid cadence %v projected a timeline", cadence)
		}
	}
}
