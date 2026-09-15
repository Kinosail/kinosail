package playback

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// RequestedDisplayCapabilities is shared semantic validation for optional
// display/decoded-audio hints. Absence preserves the existing client baseline.
// These hints never grant playback or conversion permission.
func RequestedDisplayCapabilities(request *http.Request) ([]string, int, error) {
	invalid := errors.New("playback display capabilities are invalid")
	if request == nil || request.URL == nil || len(request.URL.RawQuery) > 4096 {
		return nil, 0, invalid
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		return nil, 0, invalid
	}
	var formats []string
	if values, present := query["hdrFormats"]; present {
		if len(values) != 1 || len(values[0]) > 32 {
			return nil, 0, invalid
		}
		formats = strings.Split(values[0], ",")
		if !validHDRFormats(formats) {
			return nil, 0, invalid
		}
	}
	channels, err := requestedAudioChannels(query)
	if err != nil {
		return nil, 0, invalid
	}
	return formats, channels, nil
}

func validHDRFormats(formats []string) bool {
	seen := map[string]bool{}
	if len(formats) > 3 {
		return false
	}
	for _, format := range formats {
		if !oneOf(format, "sdr", "hdr10", "hlg") || seen[format] {
			return false
		}
		seen[format] = true
	}
	return seen["sdr"]
}

func requestedAudioChannels(query url.Values) (int, error) {
	channels := 0
	if values, present := query["maxAudioChannels"]; present {
		if len(values) != 1 || len(values[0]) != 1 || values[0][0] < '1' || values[0][0] > '8' {
			return 0, errors.New("playback display capabilities are invalid")
		}
		channels, _ = strconv.Atoi(values[0])
	}
	return channels, nil
}
