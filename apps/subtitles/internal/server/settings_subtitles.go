package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/MikeO7/kinosail-subtitles/internal/settingsstate"
	"github.com/MikeO7/kinosail-subtitles/internal/subtitlelanguage"
)

const maximumSubtitleLanguages = settingsstate.MaximumSubtitleLanguages

func (store *settingsStore) setSubtitleLanguages(languages []string) error {
	if err := store.editable("subtitles.language"); err != nil {
		return err
	}
	canonical, err := validateSubtitleLanguages(languages)
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.SubtitleLanguage = canonical[0]
	settings.SubtitleLanguages = canonical
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) setSubtitlePlan(language, preference string) error {
	if err := store.editable("subtitles.language"); err != nil {
		return err
	}
	canonical, err := validateSubtitleLanguages([]string{language})
	preference = strings.ToLower(strings.TrimSpace(preference))
	if err != nil || !oneOf(preference, "standard", "sdh") {
		return errors.New("subtitle plan is invalid")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	languages, tag := subtitleLanguages(store.value), canonical[0]
	if index := slices.Index(languages, tag); index >= 0 {
		languages = append([]string{tag}, slices.Delete(languages, index, index+1)...)
	} else {
		languages = append([]string{tag}, languages[1:]...)
	}
	settings := store.value
	settings.SubtitleLanguage, settings.SubtitleLanguages = languages[0], languages
	settings.SubtitlePreference = preference
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) setPrimarySubtitleLanguage(language string) error {
	if err := store.editable("subtitles.language"); err != nil {
		return err
	}
	canonical, err := validateSubtitleLanguages([]string{language})
	if err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	languages, tag := subtitleLanguages(store.value), canonical[0]
	if index := slices.Index(languages, tag); index >= 0 {
		languages = append([]string{tag}, slices.Delete(languages, index, index+1)...)
	} else {
		languages = append([]string{tag}, languages[1:]...)
	}
	if languages, err = validateSubtitleLanguages(languages); err != nil {
		return err
	}
	settings := store.value
	settings.SubtitleLanguage, settings.SubtitleLanguages = languages[0], languages
	if err = store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func validateSubtitleLanguages(languages []string) ([]string, error) {
	return settingsstate.ValidateSubtitleLanguages(languages)
}

func subtitleLanguagesOverlap(left, right string) bool {
	return settingsstate.Overlap(left, right)
}

func (store *settingsStore) changeSubtitleLanguages(action, language string) error { //nolint:cyclop,funlen,gocognit // All list mutation invariants are validated before persistence.
	if err := store.editable("subtitles.language"); err != nil {
		return err
	}
	if !validLanguage(language) {
		return errors.New("choose a supported subtitle language")
	}
	tag, ok := subtitlelanguage.NormalizeTag(language)
	if !ok {
		return errors.New("choose a supported subtitle language")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	languages := subtitleLanguages(store.value)
	index := slices.Index(languages, tag)
	switch action {
	case "add":
		if index >= 0 {
			return errors.New("subtitle language is already selected")
		}
		if len(languages) == maximumSubtitleLanguages {
			return errors.New("no more than 20 subtitle languages can be selected")
		}
		languages = append(languages, tag)
	case "remove":
		if index < 0 || len(languages) == 1 {
			return errors.New("at least one subtitle language is required")
		}
		languages = slices.Delete(languages, index, index+1)
	case "earlier":
		if index <= 0 {
			return errors.New("subtitle language cannot move earlier")
		}
		languages[index-1], languages[index] = languages[index], languages[index-1]
	case "later":
		if index < 0 || index == len(languages)-1 {
			return errors.New("subtitle language cannot move later")
		}
		languages[index], languages[index+1] = languages[index+1], languages[index]
	default:
		return errors.New("subtitle language action is invalid")
	}
	languages, err := validateSubtitleLanguages(languages)
	if err != nil {
		return err
	}
	settings := store.value
	settings.SubtitleLanguage = languages[0]
	settings.SubtitleLanguages = languages
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) subtitlePreference() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if store.value.SubtitlePreference == "sdh" {
		return "sdh"
	}
	return "standard"
}

func (store *settingsStore) subtitleLanguage() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return subtitleLanguages(store.value)[0]
}

func (store *settingsStore) subtitleLanguages() []string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	return subtitleLanguages(store.value)
}

func subtitleLanguages(settings installationSettings) []string {
	if languages, err := validateSubtitleLanguages(settings.SubtitleLanguages); err == nil {
		return languages
	}
	if language, ok := subtitlelanguage.NormalizeTag(settings.SubtitleLanguage); ok && validLanguage(settings.SubtitleLanguage) {
		return []string{language}
	}
	return []string{"en"}
}

type subtitleLanguagesInput struct {
	Language   json.RawMessage `json:"language"`
	Languages  json.RawMessage `json:"languages"`
	Preference json.RawMessage `json:"preference"`
}

func apiSubtitleLanguages(settings *settingsStore) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		var input subtitleLanguagesInput
		if !readSubtitleJSON(writer, request, &input) {
			return
		}
		err := settings.applySubtitleLanguages(input)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errManagedSetting) {
				status = http.StatusConflict
			}
			apiError(writer, err, status)
			return
		}
		writeJSON(writer, map[string]string{"status": "saved"}, http.StatusOK)
	}
}

func (store *settingsStore) applySubtitleLanguages(input subtitleLanguagesInput) error {
	if (input.Language == nil) == (input.Languages == nil) {
		return errors.New("choose language or languages, not both")
	}
	if input.Languages != nil && input.Preference != nil {
		return errors.New("preference requires one primary language")
	}
	if input.Language != nil {
		var language string
		if !decodeSubtitleValue(input.Language, &language) {
			return errors.New("subtitle language request is invalid")
		}
		if input.Preference == nil {
			return store.setPrimarySubtitleLanguage(language)
		}
		var preference string
		if !decodeSubtitleValue(input.Preference, &preference) {
			return errors.New("subtitle preference request is invalid")
		}
		return store.setSubtitlePlan(language, preference)
	}
	var languages []string
	if !decodeSubtitleValue(input.Languages, &languages) {
		return errors.New("subtitle language request is invalid")
	}
	return store.setSubtitleLanguages(languages)
}
