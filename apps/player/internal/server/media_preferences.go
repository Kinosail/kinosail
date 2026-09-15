package server

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode"
)

// Playback preferences belong to a Viewer. Item overrides are shared by episodes
// in the same show, while books and standalone titles keep their own choices.
type playbackPreferences struct {
	AudioTrack       string  `json:"audioTrack"`
	SubtitleTrack    string  `json:"subtitleTrack"`
	Rate             float64 `json:"rate"`
	AudioLanguage    string  `json:"audioLanguage"`
	SubtitleLanguage string  `json:"subtitleLanguage"`
	NightMode        bool    `json:"nightMode"`
	DialogueBoost    bool    `json:"dialogueBoost"`
	VolumeBoost      float64 `json:"volumeBoost"`
}

type mediaPreferences struct {
	Playback         playbackPreferences `json:"playback"`
	AutoDownloadNext int                 `json:"autoDownloadNext"`
	RemoveWatched    bool                `json:"removeWatched"`
	DownloadLimitGiB int                 `json:"downloadLimitGiB"`
	WiFiOnly         bool                `json:"wifiOnly"`
	ReaderFontSize   int                 `json:"readerFontSize"`
	ReaderTheme      string              `json:"readerTheme"`
}

var (
	preferenceLanguage  = regexp.MustCompile(`^[a-z]{2,3}(?:-[A-Za-z0-9]{2,8}){0,3}$`)
	errMediaPreferences = errors.New("invalid media preferences")
)

func defaultMediaPreferences() mediaPreferences {
	return mediaPreferences{Playback: playbackPreferences{Rate: 1, AudioLanguage: "auto", SubtitleLanguage: "auto", VolumeBoost: 1}, DownloadLimitGiB: 20, WiFiOnly: true, ReaderFontSize: 20, ReaderTheme: "auto"}
}

func preferenceLanguageValid(value string, subtitle bool) bool {
	return value == "auto" || (subtitle && value == "off") || (len(value) <= 32 && preferenceLanguage.MatchString(value))
}

// Both comparisons must succeed, which also rejects NaN and infinities.
func inMediaRange(value, minimum, maximum float64) bool {
	return value >= minimum && value <= maximum
}

func (value playbackPreferences) validate() error {
	if len(value.AudioTrack) > 256 || len(value.SubtitleTrack) > 256 || strings.ContainsFunc(value.AudioTrack+value.SubtitleTrack, unicode.IsControl) {
		return errMediaPreferences
	}
	if !inMediaRange(value.Rate, .5, 3) || !inMediaRange(value.VolumeBoost, 1, 2) ||
		!preferenceLanguageValid(value.AudioLanguage, false) || !preferenceLanguageValid(value.SubtitleLanguage, true) {
		return errMediaPreferences
	}
	return nil
}

func (value mediaPreferences) validate() error {
	if value.Playback.validate() != nil || value.AutoDownloadNext < 0 || value.AutoDownloadNext > 3 ||
		value.DownloadLimitGiB < 0 || value.DownloadLimitGiB > 8388607 || value.ReaderFontSize < 16 || value.ReaderFontSize > 32 ||
		!slices.Contains([]string{"auto", "light", "dark", "sepia"}, value.ReaderTheme) {
		return errMediaPreferences
	}
	return nil
}

func decodeMediaFields(raw []byte, target any, keys []string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return errMediaPreferences
	}
	if len(fields) != len(keys) {
		return errMediaPreferences
	}
	for _, key := range keys {
		if value, ok := fields[key]; !ok || string(value) == "null" {
			return errMediaPreferences
		}
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return errMediaPreferences
	}
	return nil
}

func (value *playbackPreferences) UnmarshalJSON(raw []byte) error {
	type plain playbackPreferences
	return decodeMediaFields(raw, (*plain)(value), []string{"rate", "audioLanguage", "subtitleLanguage", "audioTrack", "subtitleTrack", "nightMode", "dialogueBoost", "volumeBoost"})
}

func (value *mediaPreferences) UnmarshalJSON(raw []byte) error {
	type plain mediaPreferences
	return decodeMediaFields(raw, (*plain)(value), []string{"playback", "autoDownloadNext", "removeWatched", "downloadLimitGiB", "wifiOnly", "readerFontSize", "readerTheme"})
}
