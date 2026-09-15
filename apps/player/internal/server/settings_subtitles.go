package server

import (
	"errors"
	"strings"

	"golang.org/x/text/language"
)

func (store *settingsStore) setSubtitleLanguage(language string) error {
	if err := store.editable("subtitles.language"); err != nil {
		return err
	}
	language, err := canonicalSubtitleLanguage(language)
	if err != nil {
		return errors.New("subtitle language must be a two or three letter code")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	settings := store.value
	settings.SubtitleLanguage = language
	if err := store.save(settings); err != nil {
		return err
	}
	store.value = settings
	return nil
}

func (store *settingsStore) subtitleLanguage() string {
	store.mu.RLock()
	defer store.mu.RUnlock()
	if value, err := canonicalSubtitleLanguage(store.value.SubtitleLanguage); err == nil {
		return value
	}
	return "en"
}

func canonicalSubtitleLanguage(value string) (string, error) {
	base, err := language.ParseBase(strings.ToLower(strings.TrimSpace(value)))
	if err != nil {
		return "", err
	}
	return base.String(), nil
}
