// Package settingsstate validates persisted installation settings at every state boundary.
package settingsstate

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
)

const (
	// MaximumDocumentSize bounds persisted installation settings.
	MaximumDocumentSize = 1 << 20
	// MaximumSubtitleLanguages bounds ordered language preferences.
	MaximumSubtitleLanguages = 20
)

// Preferences is the canonical persisted subtitle language state.
type Preferences struct {
	Primary   string
	Languages []string
}

type document struct {
	Name                  string     `json:"name"`
	Libraries             []string   `json:"libraries"`
	RequireMFA            bool       `json:"requireMfa"`
	SessionInactiveHours  float64    `json:"sessionInactiveHours,omitempty"`
	SessionAbsoluteHours  float64    `json:"sessionAbsoluteHours,omitempty"`
	JellyfinCompatibility bool       `json:"jellyfinCompatibility"`
	HomeAssistant         bool       `json:"homeAssistant,omitempty"`
	JellyfinID            string     `json:"jellyfinId,omitempty"`
	PlaybackMode          string     `json:"playbackMode,omitempty"`
	Transcoder            string     `json:"transcoder,omitempty"`
	Codec                 string     `json:"codec,omitempty"`
	Accelerator           string     `json:"accelerator,omitempty"`
	ToneMap               bool       `json:"toneMap,omitempty"`
	Autoplay              bool       `json:"autoplay,omitempty"`
	AutoSkip              []string   `json:"autoSkip"`
	Subtitles             string     `json:"subtitles,omitempty"`
	SubtitleLanguage      string     `json:"subtitleLanguage,omitempty"`
	SubtitleLanguages     []string   `json:"subtitleLanguages,omitempty"`
	SubtitlePickerLimited bool       `json:"subtitlePickerLimited,omitempty"`
	PickerKeepForced      bool       `json:"subtitlePickerKeepForced,omitempty"`
	SubtitlePreference    string     `json:"subtitlePreference,omitempty"`
	ScanFrequency         string     `json:"scanFrequency,omitempty"`
	DLNAToken             string     `json:"dlnaToken,omitempty"`
	Navigation            []string   `json:"navigation"`
	OnboardingPending     bool       `json:"onboardingPending,omitempty"`
	UpdateChecks          bool       `json:"updateChecks"`
	Supporter             *supporter `json:"supporter,omitempty"`
}

type supporter struct {
	InstallationKey string          `json:"installationKey,omitempty"`
	PatronOrder     *supporterGrant `json:"patronOrder,omitempty"`
	LivingStandard  *supporterGrant `json:"livingStandard,omitempty"`
	PatronLevel     int             `json:"patronLevel,omitempty"`
	LivingLevel     int             `json:"livingLevel,omitempty"`
	ActivationID    string          `json:"activationId,omitempty"`
	Certificate     string          `json:"certificate,omitempty"`
	Signature       string          `json:"signature,omitempty"`
	PublicKey       string          `json:"publicKey,omitempty"`
	KeyHash         string          `json:"keyHash,omitempty"`
}

type supporterGrant struct {
	ActivationID string `json:"activationId,omitempty"`
	Certificate  string `json:"certificate,omitempty"`
	Signature    string `json:"signature,omitempty"`
	PublicKey    string `json:"publicKey,omitempty"`
	KeyHash      string `json:"keyHash,omitempty"`
}

var allowedFields = map[string]bool{
	"name": true, "libraries": true, "requiremfa": true, "sessioninactivehours": true, "sessionabsolutehours": true,
	"jellyfincompatibility": true, "homeassistant": true, "jellyfinid": true, "playbackmode": true, "transcoder": true,
	"codec": true, "accelerator": true, "tonemap": true, "autoplay": true, "autoskip": true, "subtitles": true,
	"subtitlelanguage": true, "subtitlelanguages": true, "subtitlepickerlimited": true, "subtitlepickerkeepforced": true, "subtitlepreference": true, "scanfrequency": true, "dlnatoken": true, "navigation": true,
	"onboardingpending": true, "updatechecks": true, "supporter": true,
}

// Parse validates one settings document and returns canonical language preferences.
func Parse(data []byte) (Preferences, error) {
	fields, err := validateDocument(data)
	if err != nil {
		return Preferences{}, err
	}
	if err := validatePreference(fields); err != nil {
		return Preferences{}, err
	}
	return parseLanguages(fields)
}

func validateDocument(data []byte) (map[string]json.RawMessage, error) { //nolint:cyclop // The persisted document is fully validated before decoding preferences.
	if len(data) > MaximumDocumentSize || !utf8.Valid(data) {
		return nil, errors.New("invalid installation settings")
	}
	fields, err := fields(data)
	if err != nil {
		return nil, err
	}
	for name := range fields {
		if !allowedFields[name] {
			return nil, errors.New("invalid installation settings")
		}
	}
	if value, present := fields["subtitlepickerlimited"]; present && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, errors.New("invalid installation settings")
	}
	if value, present := fields["subtitlepickerkeepforced"]; present && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, errors.New("invalid installation settings")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var settings document
	if decoder.Decode(&settings) != nil || settings.Supporter != nil && (settings.Supporter.PatronLevel < 0 || settings.Supporter.PatronLevel > 10 || settings.Supporter.LivingLevel < 0 || settings.Supporter.LivingLevel > 10) {
		return nil, errors.New("invalid installation settings")
	}
	return fields, nil
}

func validatePreference(fields map[string]json.RawMessage) error {
	if preference, present := fields["subtitlepreference"]; present {
		var value string
		if bytes.Equal(bytes.TrimSpace(preference), []byte("null")) || json.Unmarshal(preference, &value) != nil || value != "standard" && value != "sdh" {
			return errors.New("invalid persisted subtitle preference")
		}
	}
	return nil
}

func parseLanguages(fields map[string]json.RawMessage) (Preferences, error) { //nolint:cyclop // Legacy and current language fields require joint compatibility validation.
	singularRaw, singularPresent := fields["subtitlelanguage"]
	listRaw, listPresent := fields["subtitlelanguages"]
	if singularPresent && bytes.Equal(bytes.TrimSpace(singularRaw), []byte("null")) || listPresent && bytes.Equal(bytes.TrimSpace(listRaw), []byte("null")) {
		return Preferences{}, errors.New("invalid persisted subtitle languages")
	}
	var singular string
	if singularPresent && json.Unmarshal(singularRaw, &singular) != nil {
		return Preferences{}, errors.New("invalid persisted subtitle languages")
	}
	if !listPresent {
		return parseSingularLanguage(singular, singularPresent)
	}
	var languages []string
	if json.Unmarshal(listRaw, &languages) != nil {
		return Preferences{}, errors.New("invalid persisted subtitle languages")
	}
	canonical, err := ValidateSubtitleLanguages(languages)
	if err != nil {
		return Preferences{}, errors.New("invalid persisted subtitle languages")
	}
	if singularPresent {
		primary, primaryErr := ValidateSubtitleLanguages([]string{singular})
		if primaryErr != nil || primary[0] != canonical[0] {
			return Preferences{}, errors.New("persisted subtitle language conflicts with the preference list")
		}
	}
	return Preferences{Primary: canonical[0], Languages: canonical}, nil
}

func parseSingularLanguage(singular string, present bool) (Preferences, error) {
	if !present {
		return Preferences{Primary: "en", Languages: []string{"en"}}, nil
	}
	canonical, err := ValidateSubtitleLanguages([]string{singular})
	if err != nil {
		return Preferences{}, errors.New("invalid persisted subtitle languages")
	}
	return Preferences{Primary: canonical[0], Languages: canonical}, nil
}

// Validate checks one installation settings document.
func Validate(data []byte) error {
	_, err := Parse(data)
	return err
}

// ValidateSubtitleLanguages canonicalizes one complete ordered preference list.
func ValidateSubtitleLanguages(languages []string) ([]string, error) {
	if len(languages) == 0 || len(languages) > MaximumSubtitleLanguages {
		return nil, errors.New("choose 1 to 20 subtitle languages")
	}
	canonical := make([]string, 0, len(languages))
	for _, value := range languages {
		tag, ok := subtitlelanguage.NormalizeTag(value)
		if !ok {
			return nil, errors.New("choose a supported subtitle language")
		}
		if slices.Contains(canonical, tag) {
			return nil, errors.New("subtitle languages must be unique")
		}
		for _, selected := range canonical {
			if Overlap(selected, tag) {
				return nil, errors.New("a base language and its variant cannot both be selected")
			}
		}
		canonical = append(canonical, tag)
	}
	return canonical, nil
}

// Overlap reports whether a base language and one of its variants conflict.
func Overlap(left, right string) bool {
	leftBase, rightBase := strings.SplitN(left, "-", 2)[0], strings.SplitN(right, "-", 2)[0]
	return leftBase == rightBase && (left == leftBase || right == rightBase)
}

func fields(data []byte) (map[string]json.RawMessage, error) { //nolint:cyclop // Strict document parsing checks all structural boundaries together.
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, errors.New("invalid installation settings")
	}
	result := make(map[string]json.RawMessage)
	for decoder.More() {
		keyToken, keyErr := decoder.Token()
		key, ok := keyToken.(string)
		folded := strings.ToLower(key)
		if keyErr != nil || !ok || folded == "" {
			return nil, errors.New("invalid installation settings")
		}
		if _, duplicate := result[folded]; duplicate {
			return nil, errors.New("duplicate installation setting")
		}
		var raw json.RawMessage
		if decoder.Decode(&raw) != nil {
			return nil, errors.New("invalid installation settings")
		}
		result[folded] = raw
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("invalid installation settings")
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("invalid installation settings")
	}
	return result, nil
}
