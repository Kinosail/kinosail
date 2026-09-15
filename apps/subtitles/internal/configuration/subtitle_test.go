package configuration

import "testing"

func TestSubtitleLanguageConfigurationRejectsProviderSpellings(t *testing.T) {
	for _, value := range []string{"ea", "sp", "at", "pm", "zh_bg", "English", "br_pt"} {
		if validSubtitleLanguage(value) {
			t.Errorf("provider spelling %q was accepted", value)
		}
	}
	for _, value := range []string{"en", "ES-419", "pt-BR", "zh-Hant"} {
		if !validSubtitleLanguage(value) {
			t.Errorf("selectable tag %q was rejected", value)
		}
	}
}
