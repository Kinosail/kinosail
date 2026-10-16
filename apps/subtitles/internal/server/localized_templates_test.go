package server

import "testing"

func TestEveryLocalizedTemplateBuildsForEverySupportedLanguage(t *testing.T) {
	t.Parallel()
	views := registeredLocalizedTemplates()
	if len(views) < 25 {
		t.Fatalf("only %d localized templates registered", len(views))
	}
	for _, view := range views {
		if err := view.Validate(); err != nil {
			t.Error(err)
		}
	}
}
