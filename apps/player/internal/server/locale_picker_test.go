package server

import (
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestLanguagePickerShowsEverySupportedLocale(t *testing.T) {
	t.Parallel()
	servertest.LanguagePickerShowsEverySupportedLocale(t, New(Config{DataDir: t.TempDir()}), supportedLanguages)
}

func TestLanguagePickerTemplatesUseCurrentStylesheet(t *testing.T) {
	t.Parallel()
	const stylesheet = `/static/app.css?v=skeleton-2`
	for name, source := range map[string]string{
		"account":        accountHTML,
		"home":           homeTemplateSource(),
		"language":       languageHTML,
		"login":          profileLoginHTML,
		"passkey prompt": passkeyPromptHTML,
		"settings":       updateChoicePage(settingsHTML),
		"setup":          setupPage,
	} {
		currentStylesheet := stylesheet
		if name == "home" {
			currentStylesheet = `/static/app.css?v=skeleton-2`
		}
		if name == "settings" {
			currentStylesheet = `/static/app.css?v=skeleton-2`
		}
		if !strings.Contains(source, "languagePicker") {
			t.Fatalf("%s does not contain a language picker", name)
		}
		if !strings.Contains(source, currentStylesheet) {
			t.Errorf("%s does not reference %s", name, currentStylesheet)
		}
	}
}

func TestAutomaticDetectionCoversEverySupportedLanguage(t *testing.T) {
	t.Parallel()
	servertest.AutomaticDetectionCoversEverySupportedLanguage(t, New(Config{DataDir: t.TempDir(), RequireAuth: true}), supportedLanguages, automaticLanguage)
}
