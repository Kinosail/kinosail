package configuration_test

import (
	"maps"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestOpenSubtitlesConfigurationRequiresOneCompleteSafeSet(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"KINOSAIL_OPENSUBTITLES_URL":      "https://api.opensubtitles.com/api/v1",
		"KINOSAIL_OPENSUBTITLES_API_KEY":  "app-key",
		"KINOSAIL_OPENSUBTITLES_USERNAME": "owner",
		"KINOSAIL_OPENSUBTITLES_PASSWORD": "password",
	}
	if _, err := configuration.Load(t.TempDir(), "", lookup(valid)); err != nil {
		t.Fatalf("valid OpenSubtitles configuration: %v", err)
	}
	for name, change := range map[string]map[string]string{
		"missing API key":  {"KINOSAIL_OPENSUBTITLES_API_KEY": ""},
		"missing username": {"KINOSAIL_OPENSUBTITLES_USERNAME": ""},
		"missing password": {"KINOSAIL_OPENSUBTITLES_PASSWORD": ""},
		"insecure URL":     {"KINOSAIL_OPENSUBTITLES_URL": "http://api.opensubtitles.com/api/v1"},
		"URL credentials":  {"KINOSAIL_OPENSUBTITLES_URL": "https://owner@api.opensubtitles.com/api/v1"},
		"URL query":        {"KINOSAIL_OPENSUBTITLES_URL": "https://api.opensubtitles.com/api/v1?x=1"},
		"spaced username":  {"KINOSAIL_OPENSUBTITLES_USERNAME": " owner"},
		"oversized secret": {"KINOSAIL_OPENSUBTITLES_PASSWORD": strings.Repeat("x", 4097)},
		"key control":      {"KINOSAIL_OPENSUBTITLES_API_KEY": "key\nvalue"},
		"username control": {"KINOSAIL_OPENSUBTITLES_USERNAME": "owner\tname"},
		"password control": {"KINOSAIL_OPENSUBTITLES_PASSWORD": "pass\x00word"},
	} {
		t.Run(name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			maps.Copy(values, valid)
			maps.Copy(values, change)
			if _, err := configuration.Load(t.TempDir(), "", lookup(values)); err == nil {
				t.Fatal("invalid OpenSubtitles configuration was accepted")
			}
		})
	}
}

func TestSubtitleLanguageAcceptsRegionsAndRejectsAmbiguity(t *testing.T) {
	t.Parallel()
	for _, language := range []string{"en", "EN", "eng", "pt-br", "PT-br", "pt-BR", "zh-Hans", "ZH-hant", "es-419"} {
		if _, err := configuration.Load(t.TempDir(), "", lookup(map[string]string{"KINOSAIL_SUBTITLE_LANGUAGE": language})); err != nil {
			t.Fatalf("language %q: %v", language, err)
		}
	}
	for _, language := range []string{"e", "english", "pt-bra", "en-us-extra", " en"} {
		if _, err := configuration.Load(t.TempDir(), "", lookup(map[string]string{"KINOSAIL_SUBTITLE_LANGUAGE": language})); err == nil {
			t.Fatalf("invalid language %q was accepted", language)
		}
	}
}
