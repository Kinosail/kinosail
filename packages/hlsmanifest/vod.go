package hlsmanifest

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

// CompleteVOD preserves observed segments and projects only an established cadence.
func CompleteVOD(manifest []byte, duration, cadence float64) ([]byte, bool) {
	if !bytes.Contains(manifest, []byte("#EXT-X-PLAYLIST-TYPE:EVENT")) {
		return manifest, true
	}
	if invalidHLSSegmentDuration(cadence) {
		return manifest, false
	}
	if playback.PlaylistHas(manifest, "#EXT-X-ENDLIST") {
		return bytes.Replace(manifest, []byte("#EXT-X-PLAYLIST-TYPE:EVENT"), []byte("#EXT-X-PLAYLIST-TYPE:VOD"), 1), true
	}
	if invalidHLSVODDuration(duration) {
		return manifest, false
	}
	prefix, valid := observedVODPrefix(manifest, cadence)
	if !valid {
		return manifest, false
	}
	if !prefix.ready {
		return manifest, false
	}
	if prefix.duration > duration {
		return manifest, false
	}
	cadence = prefix.cadence
	count, valid := vodProjectionCount(prefix, duration)
	if !valid {
		return manifest, false
	}
	for index := range count {
		length := min(cadence, duration-prefix.duration-float64(index)*cadence)
		prefix.lines = append(prefix.lines, "#EXTINF:"+strconv.FormatFloat(length, 'f', 6, 64)+",", fmt.Sprintf("segment-%05d.m4s", prefix.segments+index))
	}
	result := strings.Join(append(prefix.lines, "#EXT-X-ENDLIST", ""), "\n")
	return []byte(strings.Replace(result, "#EXT-X-PLAYLIST-TYPE:EVENT", "#EXT-X-PLAYLIST-TYPE:VOD", 1)), true
}

func vodProjectionCount(prefix vodPrefix, duration float64) (int, bool) {
	if prefix.segments > 100_000 {
		return 0, false
	}
	count := int(math.Ceil((duration - prefix.duration) / prefix.cadence))
	if count < 0 || count > 100_000-prefix.segments {
		return 0, false
	}
	return count, true
}

type vodPrefix struct {
	lines    []string
	duration float64
	cadence  float64
	segments int
	ready    bool
}

func observedVODPrefix(manifest []byte, cadence float64) (vodPrefix, bool) {
	prefix := vodPrefix{lines: strings.Split(strings.TrimRight(string(manifest), "\n"), "\n")}
	previous := 0.0
	for _, line := range prefix.lines {
		if !strings.HasPrefix(line, "#EXTINF:") {
			continue
		}
		value, _, _ := strings.Cut(strings.TrimPrefix(line, "#EXTINF:"), ",")
		observed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return vodPrefix{}, false
		}
		if invalidHLSSegmentDuration(observed) {
			return vodPrefix{}, false
		}
		prefix.duration += observed
		prefix.segments++
		rounded := math.Round(observed)
		if !prefix.ready && rounded > 0 {
			if rounded == math.Round(cadence) {
				prefix.cadence, prefix.ready = cadence, true
			} else if rounded == math.Round(previous) {
				prefix.cadence, prefix.ready = (previous+observed)/2, true
			}
		}
		previous = observed
	}
	return prefix, prefix.segments > 0
}

func invalidHLSVODDuration(duration float64) bool {
	if math.IsNaN(duration) {
		return true
	}
	if duration <= 0 {
		return true
	}
	return duration > 7*24*60*60
}

func invalidHLSSegmentDuration(duration float64) bool {
	if math.IsNaN(duration) {
		return true
	}
	if duration <= 0 {
		return true
	}
	return duration > 60
}
