package playback

import (
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/MikeO7/kinosail/packages/library"
)

var webVTTMarkup = regexp.MustCompile(`</?(?:c|v|lang|ruby|rt)(?:[.\s][^>]*)?>|<\d{2}:[\d:.]+>`)

func JellyfinSidecarBase(facts MediaFacts) int {
	base := 1
	for _, track := range facts.Audio {
		base = max(base, track.SourceIndex+1)
	}
	for _, track := range facts.Subtitles {
		if !track.External {
			base = max(base, track.SourceIndex+1)
		}
	}
	return base
}

func JellyfinIntent(facts MediaFacts, input JellyfinPlaybackRequest) NetworkIntent {
	intent := NetworkIntent{MaxBitrate: input.MaxStreamingBitrate}
	if input.AudioStreamIndex != nil && *input.AudioStreamIndex >= 0 {
		index := -1
		for _, track := range facts.Audio {
			if track.SourceIndex == *input.AudioStreamIndex {
				index = track.Index
			}
		}
		intent.AudioIndex = &index
	}
	if input.SubtitleStreamIndex != nil && *input.SubtitleStreamIndex >= 0 {
		index, base := -1, JellyfinSidecarBase(facts)
		for _, track := range facts.Subtitles {
			if !track.External && track.SourceIndex == *input.SubtitleStreamIndex || track.External && base+track.ExternalIndex == *input.SubtitleStreamIndex {
				index = track.Index
			}
		}
		intent.SubtitleIndex = &index
	}
	return intent
}

func JellyfinSubtitleFormat(profiles []JellyfinSubtitleProfile) string {
	format := "vtt"
	for _, profile := range profiles {
		if !strings.EqualFold(profile.Method, "External") {
			continue
		}
		switch subtitleCodecName(profile.Format) {
		case "vtt":
			return "vtt"
		case "srt":
			format = "srt"
		}
	}
	return format
}

func jellyfinSidecarStreams(item library.Item, facts MediaFacts, format, query string) []any {
	if format != "srt" {
		format = "vtt"
	}
	streams, base := make([]any, 0, len(item.Subtitles)), JellyfinSidecarBase(facts)
	for index := range item.Subtitles {
		stream := map[string]any{
			"Index": base + index, "Type": "Subtitle", "Codec": format, "DisplayTitle": "Subtitle " + strconv.Itoa(index+1),
			"IsExternal": true, "IsTextSubtitleStream": true, "SupportsExternalStream": true, "DeliveryMethod": "External",
			"DeliveryUrl": jellyfinSubtitleURL(item, base+index, format, query),
		}
		for _, track := range facts.Subtitles {
			if track.External && track.ExternalIndex == index {
				stream["DisplayTitle"], stream["IsForced"] = jellyfinSubtitleTitle(track), track.Forced
				if track.Language != "" && track.Language != "und" {
					stream["Language"] = track.Language
				}
			}
		}
		streams = append(streams, stream)
	}
	return streams
}

func jellyfinSubtitleTitle(track SubtitleFacts) string {
	role := track.Role
	if role == "translation" {
		role = ""
	}
	return SubtitleLabel(SubtitleOption{Language: track.Language, Role: role, Title: track.Title, Forced: track.Forced, Converted: track.Converted, External: track.External})
}

func jellyfinSubtitleURL(item library.Item, index int, format, query string) string {
	if format != "srt" {
		format = "vtt"
	}
	return "/Videos/" + JellyfinID(item.ID) + "/" + JellyfinID(item.ID) + "/Subtitles/" + strconv.Itoa(index) + "/Stream." + format + query
}

func subtitleCodecName(codec string) string {
	switch codec = Lower(codec); codec {
	case "subrip":
		return "srt"
	case "webvtt":
		return "vtt"
	default:
		return codec
	}
}

func textSubtitleDeliverable(client ClientCapabilities, codec string) bool {
	for _, supported := range client.TextSubtitleCodecs {
		name := subtitleCodecName(supported)
		if name == subtitleCodecName(codec) || client.SupportsExternalSubtitles && (name == "vtt" || name == "srt") {
			return true
		}
	}
	return false
}

func WebVTTToSubRip(data []byte) []byte {
	var output strings.Builder
	cue := 0
	for block := range strings.SplitSeq(normalizeSubtitleNewlines(data), "\n\n") {
		lines := strings.Split(strings.Trim(block, "\n"), "\n")
		timing := -1
		for index, line := range lines {
			if subtitleTimingFields(line) != nil {
				timing = index
				break
			}
		}
		if timing < 0 || timing+1 >= len(lines) {
			continue
		}
		fields := subtitleTimingFields(lines[timing])
		start, startErr := ParseVTTTime(fields[0])
		end, endErr := ParseVTTTime(fields[2])
		if startErr != nil || endErr != nil || end <= start {
			continue
		}
		cue++
		output.WriteString(strconv.Itoa(cue) + "\n" + subRipTime(start) + " --> " + subRipTime(end) + "\n")
		for _, line := range lines[timing+1:] {
			output.WriteString(html.UnescapeString(webVTTMarkup.ReplaceAllString(line, "")) + "\n")
		}
		output.WriteString("\n")
	}
	return []byte(output.String())
}

func subRipTime(seconds float64) string {
	return strings.Replace(FormatVTTTime(seconds), ".", ",", 1)
}
