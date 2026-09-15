package server

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestLanguagePickerShowsEverySupportedLocale(t *testing.T) {
	t.Parallel()
	servertest.LanguagePickerShowsEverySupportedLocale(t, New(Config{DataDir: t.TempDir()}), supportedLanguages)
}

func TestAutomaticDetectionCoversEverySupportedLanguage(t *testing.T) {
	t.Parallel()
	servertest.AutomaticDetectionCoversEverySupportedLanguage(t, New(Config{DataDir: t.TempDir(), RequireAuth: true}), supportedLanguages, automaticLanguage)
}
