package mediaprobe

import (
	"math"
	"strconv"
	"strings"
)

type ReplayGain struct {
	Track    float64
	Album    float64
	TrackSet bool
	AlbumSet bool
}

// ParseReplayGain normalizes standard and R128 gain tags.
func ParseReplayGain(tags map[string]string) ReplayGain {
	result := ReplayGain{}
	if result.Track, result.TrackSet = gainTag(tags, "replaygain_track_gain", false); !result.TrackSet {
		result.Track, result.TrackSet = gainTag(tags, "r128_track_gain", true)
	}
	if result.Album, result.AlbumSet = gainTag(tags, "replaygain_album_gain", false); !result.AlbumSet {
		result.Album, result.AlbumSet = gainTag(tags, "r128_album_gain", true)
	}
	return result
}

func gainTag(tags map[string]string, name string, r128 bool) (float64, bool) { //nolint:cyclop // Tag formats are normalized at this parser seam.
	value := strings.TrimSpace(tagValue(tags, name))
	if value == "" {
		return 0, false
	}
	parts := strings.Fields(value)
	if r128 {
		if len(parts) != 1 {
			return 0, false
		}
	} else if len(parts) > 2 || (len(parts) == 2 && !strings.EqualFold(parts[1], "db")) {
		return 0, false
	}
	gain, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || math.IsNaN(gain) || math.IsInf(gain, 0) {
		return 0, false
	}
	if r128 {
		gain /= 256
	}
	if gain < -128 || gain > 128 {
		return 0, false
	}
	return gain, true
}
