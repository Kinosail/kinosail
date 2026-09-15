package playback

import (
	"errors"
	"net/http"
	"strings"
)

// RequestedVideoCodecs parses the bounded browser capability query.
func RequestedVideoCodecs(request *http.Request) ([]string, error) {
	return requestedCodecs(request, "video", []string{"h264", "hevc", "av1", "vp9"})
}

func RequestedAudioCodecs(request *http.Request) ([]string, error) {
	return requestedCodecs(request, "audio", []string{"aac", "mp3", "opus", "vorbis", "ac3", "eac3"})
}

func requestedCodecs(request *http.Request, kind string, allowed []string) ([]string, error) {
	invalid := errors.New(kind + " codecs are invalid")
	if request == nil || request.URL == nil {
		return nil, invalid
	}
	values, present := request.URL.Query()[kind+"Codecs"]
	if !present {
		return nil, nil
	}
	if len(values) != 1 || values[0] == "" || len(values[0]) > 64 {
		return nil, invalid
	}
	return parseRequestedCodecs(values[0], allowed, invalid)
}

func SubtitleRoleLabel(role string) string {
	if role == "" {
		return "Subtitles"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func parseRequestedCodecs(raw string, allowed []string, invalid error) ([]string, error) {
	codecs, seen := strings.Split(raw, ","), map[string]bool{}
	if len(codecs) > len(allowed) {
		return nil, invalid
	}
	for _, codec := range codecs {
		if codec != Lower(codec) || !oneOf(codec, allowed...) || seen[codec] {
			return nil, invalid
		}
		seen[codec] = true
	}
	return codecs, nil
}
