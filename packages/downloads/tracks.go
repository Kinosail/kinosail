package downloads

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

type TrackSelection struct {
	Audio     []int `json:"audio"`
	Subtitles []int `json:"subtitles"`
}

// A nil selection means every track. Explicit empty arrays mean none.
func validateSelection(selection *TrackSelection) bool {
	if selection == nil {
		return true
	}
	if selection.Audio == nil || selection.Subtitles == nil {
		return false
	}
	for _, indices := range [][]int{selection.Audio, selection.Subtitles} {
		if len(indices) > 256 {
			return false
		}
		seen := map[int]bool{}
		for _, index := range indices {
			if index < 0 || index > 4095 || seen[index] {
				return false
			}
			seen[index] = true
		}
	}
	return len(selection.Audio) <= 32
}

func includesTrack(indices []int, index int) bool {
	if indices == nil {
		return true
	}
	for _, value := range indices {
		if value == index {
			return true
		}
	}
	return false
}

type TrackOption struct {
	Index int    `json:"index"`
	Label string `json:"label"`
}
type TrackOptions struct {
	Audio     []TrackOption `json:"audio"`
	Subtitles []TrackOption `json:"subtitles"`
}

func (manager *Manager) Tracks(item library.Item) (TrackOptions, error) {
	if item.Kind != "video" || manager.inspect == nil {
		return TrackOptions{}, errors.New("track information is unavailable")
	}
	facts := manager.inspect(manager.ctx, item)
	if len(facts.Audio) > 32 || len(facts.Subtitles) > 256 {
		return TrackOptions{}, errors.New("too many media tracks")
	}
	result := TrackOptions{Audio: []TrackOption{}, Subtitles: []TrackOption{}}
	for _, track := range facts.Audio {
		result.Audio = append(result.Audio, TrackOption{track.Index, trackLabel("Audio", track.Index, track.Language, track.Role)})
	}
	for _, track := range facts.Subtitles {
		result.Subtitles = append(result.Subtitles, TrackOption{track.Index, trackLabel("Subtitles", track.Index, track.Language, track.Role)})
	}
	return result, nil
}

func trackLabel(kind string, index int, language, role string) string {
	label := kind + " " + strconv.Itoa(index+1)
	if language != "" && len(language) <= 64 && !strings.ContainsAny(language, "\x00\r\n") {
		label += " · " + language
	}
	if role != "" && len(role) <= 64 && !strings.ContainsAny(role, "\x00\r\n") {
		label += " · " + role
	}
	return label
}

// Resolve all indices and sidecar paths against scanned facts before execution.
// Matroska preserves bitmap subtitles and styled text without silent loss.
func resolveTracks(facts playback.MediaFacts, item library.Item, selection *TrackSelection) ([]string, []string, bool, error) {
	if !validTrackRequest(facts, item, selection) {
		return nil, nil, false, errors.New("download tracks are invalid")
	}
	var audio, subtitles []int
	if selection != nil {
		audio, subtitles = selection.Audio, selection.Subtitles
	}
	inputs, maps := []string{}, []string{"-map", "0:v:0"}
	maps, foundAudio, err := audioTrackMaps(facts.Audio, audio, maps)
	if err != nil {
		return nil, nil, false, err
	}
	inputs, maps, foundSubtitle, matroska, err := subtitleTrackMaps(facts.Subtitles, item.Subtitles, subtitles, inputs, maps)
	if err != nil {
		return nil, nil, false, err
	}
	for _, index := range audio {
		if !foundAudio[index] {
			return nil, nil, false, errors.New("download audio is unavailable")
		}
	}
	for _, index := range subtitles {
		if !foundSubtitle[index] {
			return nil, nil, false, errors.New("download subtitle is unavailable")
		}
	}
	return inputs, maps, matroska, nil
}

func audioTrackMaps(tracks []playback.AudioFacts, audio []int, maps []string) ([]string, map[int]bool, error) {
	foundAudio := map[int]bool{}
	for _, track := range tracks {
		if track.Index < 0 || track.Index > 4095 || track.SourceIndex < 0 || track.SourceIndex > 4095 || foundAudio[track.Index] {
			return nil, nil, errors.New("download audio is invalid")
		}
		foundAudio[track.Index] = true
		if includesTrack(audio, track.Index) {
			maps = append(maps, "-map", fmt.Sprintf("0:%d", track.SourceIndex))
		}
	}
	return maps, foundAudio, nil
}

func appendSubtitleTrack(track playback.SubtitleFacts, sidecars, inputs, maps []string) ([]string, []string, error) {
	if track.External {
		if track.ExternalIndex < 0 || track.ExternalIndex >= len(sidecars) {
			return nil, nil, errors.New("download subtitle is unavailable")
		}
		path := sidecars[track.ExternalIndex]
		if path == "" || len(path) > 4096 || strings.ContainsAny(path, "\x00\r\n") {
			return nil, nil, errors.New("download subtitle is invalid")
		}
		inputs = append(inputs, "-i", path)
		maps = append(maps, "-map", fmt.Sprintf("%d:0", len(inputs)/2))
	} else {
		if track.SourceIndex < 0 || track.SourceIndex > 4095 {
			return nil, nil, errors.New("download subtitle is invalid")
		}
		maps = append(maps, "-map", fmt.Sprintf("0:%d", track.SourceIndex))
	}
	return inputs, maps, nil
}

func subtitleTrackMaps(tracks []playback.SubtitleFacts, sidecars []string, subtitles []int, inputs, maps []string) ([]string, []string, map[int]bool, bool, error) {
	foundSubtitle := map[int]bool{}
	matroska := false
	for _, track := range tracks {
		if track.Index < 0 || track.Index > 4095 || foundSubtitle[track.Index] {
			return nil, nil, nil, false, errors.New("download subtitles are invalid")
		}
		foundSubtitle[track.Index] = true
		if !includesTrack(subtitles, track.Index) {
			continue
		}
		if !track.Text || track.Codec == "ass" || track.Codec == "ssa" {
			matroska = true
		}
		var err error
		inputs, maps, err = appendSubtitleTrack(track, sidecars, inputs, maps)
		if err != nil {
			return nil, nil, nil, false, err
		}
	}
	return inputs, maps, foundSubtitle, matroska, nil
}

func validTrackRequest(facts playback.MediaFacts, item library.Item, selection *TrackSelection) bool {
	return validateSelection(selection) && len(facts.Audio) <= 32 && len(facts.Subtitles) <= 256 && len(item.Subtitles) <= 256
}
