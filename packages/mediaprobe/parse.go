package mediaprobe

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"

	markerlogic "github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/metadata"
)

func parse(data []byte) (Result, bool) { //nolint:cyclop,funlen,gocognit // Preserve rejection separately from empty normalized facts.
	if len(data) > maximumProbeOutput {
		return Result{}, false
	}
	var output probeOutput
	if json.Unmarshal(data, &output) != nil || len(output.Streams) > 256 || len(output.Chapters) > 4096 {
		return Result{}, false
	}
	parts, video, audioFacts, subtitleFacts, audio := parseStreams(output.Streams)
	duration, _ := strconv.ParseFloat(output.Format.Duration, 64)
	bitrate, _ := strconv.ParseInt(output.Format.Bitrate, 10, 64)
	if duration > 0 {
		parts = append(parts, time.Duration(duration*float64(time.Second)).Round(time.Second).String())
	}
	chapters, markers := parseChapters(output.Chapters)
	tags := output.Format.Tags
	container, _, _ := strings.Cut(output.Format.Name, ",")
	audioCodec := ""
	if len(audioFacts) > 0 {
		audioCodec = audioFacts[0].Codec
	}
	result := Result{Summary: strings.Join(parts, " · "), Container: lower(container), Bitrate: bitrate, Video: video, VideoCodec: video.Codec, AudioCodec: audioCodec, Audio: audio, AudioFacts: audioFacts, SubtitleFacts: subtitleFacts, Duration: duration, Chapters: chapters, Markers: markers, Tags: Tags{tagValue(tags, "title"), tagValue(tags, "artist"), tagValue(tags, "album_artist"), tagValue(tags, "album"), tagValue(tags, "genre"), tagNumber(tags, "disc"), tagNumber(tags, "track")}, ReplayGain: ParseReplayGain(tags)}
	if !validProbeResult(result) {
		return Result{}, false
	}
	return result, true
}

func tagValue(tags map[string]string, name string) string {
	for key, value := range tags {
		if strings.EqualFold(key, name) {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func tagNumber(tags map[string]string, name string) int {
	value, _, _ := strings.Cut(tagValue(tags, name), "/")
	number, _ := strconv.Atoi(value)
	return number
}

type (
	Chapter = metadata.Chapter
	Marker  = markerlogic.Marker
)

func automaticallySkippable(marker Marker) bool {
	return marker.Source == "manual" || marker.Source == "chapter" || marker.Source == "fingerprint"
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func lower(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func parseChapters(raw []struct {
	Start string            `json:"start_time"`
	End   string            `json:"end_time"`
	Tags  map[string]string `json:"tags"`
},
) ([]Chapter, []Marker) {
	chapters := make([]Chapter, 0, len(raw))
	for index, value := range raw {
		start, startErr := strconv.ParseFloat(value.Start, 64)
		end, endErr := strconv.ParseFloat(value.End, 64)
		if startErr != nil || endErr != nil || start < 0 || end <= start {
			continue
		}
		title := strings.TrimSpace(value.Tags["title"])
		if title == "" {
			title = "Chapter " + strconv.Itoa(index+1)
		}
		chapters = append(chapters, Chapter{Index: index, Start: start, End: end, Title: title})
	}
	return chapters, markerlogic.DetectPlaybackMarkers(chapters)
}

func probeAudioLabel(stream probeStream, index int) string {
	if label := strings.TrimSpace(stream.Tags["title"]); label != "" {
		return label
	}
	if language := strings.ToUpper(stream.Tags["language"]); language != "" {
		return language
	}
	return "Audio " + strconv.Itoa(index+1)
}
