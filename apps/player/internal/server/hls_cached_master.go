package server

import (
	"bytes"
	"errors"
	"strings"

	"github.com/MikeO7/kinosail/packages/playback"
)

// Repair masters retained before the unfinished-bandwidth floor was introduced.
// Keep their rendition set and media fragments, including after a seek restart.
func refreshCachedVideoHLSMaster(playlist string, facts MediaFacts, recipe hlsRecipe, identity string) error {
	if facts.Kind != "video" || recipe.mode != "transcode" {
		return nil
	}
	manifest, err := playback.ReadHLSPlaylist(playlist)
	if err != nil {
		return err
	}
	if playback.PlaylistHas(manifest, playback.HLSBandwidthPolicyMarker) {
		return nil
	}
	qualities, err := cachedVideoHLSQualities(manifest, hlsTranscodeQualities(facts, recipe))
	if err != nil {
		return err
	}
	write := func(path string, data []byte) error {
		if bytes.Equal(manifest, data) {
			return nil
		}
		return writeAtomicFile(path, data)
	}
	independent := playback.PlaylistHas(manifest, "#EXT-X-INDEPENDENT-SEGMENTS")
	return playback.WriteMaster(playlist, identity, "", qualities, independent, write)
}

func cachedVideoHLSQualities(manifest []byte, available []PlaybackQuality) ([]PlaybackQuality, error) {
	var qualities []PlaybackQuality
	for _, line := range strings.Split(string(manifest), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		found := false
		for _, quality := range available {
			if line == quality.Label+"/index.m3u8" {
				qualities = append(qualities, quality)
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("cached HLS rendition is invalid")
		}
	}
	return qualities, nil
}
