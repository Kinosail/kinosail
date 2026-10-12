package hlsmanifest

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

// CompleteCuts projects a caller-certified cut map only when the observed
// prefix agrees. Cuts contains each segment start followed by the genuine EOF.
// The caller owns source identity, generated-media clocks, and delivery readiness.
func CompleteCuts(manifest []byte, cuts []float64) ([]byte, bool) {
	if !playback.PlaylistHas(manifest, "#EXT-X-PLAYLIST-TYPE:EVENT") ||
		!bytes.Contains(manifest, []byte("#EXT-X-MAP:URI=\"init.mp4\"")) || !MatchesCuts(manifest, cuts) {
		return manifest, false
	}
	target := 1.0
	for number := 0; number+1 < len(cuts); number++ {
		target = max(target, math.Ceil(cuts[number+1]-cuts[number]))
	}
	var output strings.Builder
	fmt.Fprintf(&output, "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-TARGETDURATION:%.0f\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-MAP:URI=\"init.mp4\"\n", target)
	for number := 0; number+1 < len(cuts); number++ {
		fmt.Fprintf(&output, "#EXTINF:%.6f,\nsegment-%05d.m4s\n", cuts[number+1]-cuts[number], number)
	}
	output.WriteString("#EXT-X-ENDLIST\n")
	return []byte(output.String()), true
}

// MatchesCuts checks a bounded sequential prefix, including its final duration
// when complete. It does not certify media bytes or source identity.
func MatchesCuts(manifest []byte, cuts []float64) bool {
	if len(manifest) > 1<<20 || !validCuts(cuts) {
		return false
	}
	length, observed := 0.0, 0
	for _, line := range strings.Split(string(manifest), "\n") {
		var err error
		length, err = cutLength(line, length)
		if err != nil {
			return false
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !matchesCut(cuts, observed, line, length) {
			return false
		}
		observed++
		length = 0
	}
	return observed > 0 && (!playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") || observed+1 == len(cuts))
}

func matchesCut(cuts []float64, number int, name string, length float64) bool {
	return number+1 < len(cuts) && name == fmt.Sprintf("segment-%05d.m4s", number) &&
		MatchesCutLength(cuts[number+1]-cuts[number], length, number+2 == len(cuts))
}

func cutLength(line string, previous float64) (float64, error) {
	if !strings.HasPrefix(line, "#EXTINF:") {
		return previous, nil
	}
	value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
	return strconv.ParseFloat(value, 64)
}

func validCuts(cuts []float64) bool {
	if len(cuts) < 2 || len(cuts) > 4097 {
		return false
	}
	for number, point := range cuts {
		if point < 0 || point > 7*24*60*60 || math.IsNaN(point) || math.IsInf(point, 0) ||
			number > 0 && invalidHLSSegmentDuration(point-cuts[number-1]) {
			return false
		}
	}
	return true
}

// MatchesCutLength retains the demuxer's bounded final-GOP rounding tolerance.
func MatchesCutLength(expected, observed float64, final bool) bool {
	if invalidHLSSegmentDuration(expected) || observed <= 0 {
		return false
	}
	tolerance := 0.002
	if final {
		tolerance = min(1, max(0.1, expected*0.05))
	}
	return math.Abs(observed-expected) <= tolerance
}
