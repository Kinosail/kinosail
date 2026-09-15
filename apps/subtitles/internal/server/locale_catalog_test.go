package server

import (
	"encoding/json"
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestLocaleCatalogsAreComplete(t *testing.T) {
	t.Parallel()
	servertest.LocaleCatalogsAreComplete(t, testSupportedLocales(), readTestCatalog)
}

func TestSupporterLocaleCatalogsCoverEnglishSpanishAndArabic(t *testing.T) {
	english := readNamedTestCatalog(t, "supporter.en.json")
	for _, tag := range []string{"es", "ar"} {
		catalog := readNamedTestCatalog(t, "supporter."+tag+".json")
		if len(catalog) != len(english) {
			t.Fatalf("supporter %s catalog has %d messages; want %d", tag, len(catalog), len(english))
		}
		for id := range english {
			if catalog[id] == "" {
				t.Fatalf("supporter %s catalog is missing %q", tag, id)
			}
		}
	}
}

func TestSupportedLanguageCoverageIncludesJellyfinLocaleSet(t *testing.T) {
	t.Parallel()
	servertest.SupportedLanguageCoverageIncludesJellyfinLocaleSet(t, testSupportedLocales())
}

func readTestCatalog(t *testing.T, tag string) map[string]string {
	t.Helper()
	return readNamedTestCatalog(t, "active."+tag+".json")
}

func readNamedTestCatalog(t *testing.T, name string) map[string]string {
	t.Helper()
	data, err := localeCatalogs.ReadFile("locales/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var messages []catalogMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		t.Fatal(err)
	}
	result := make(map[string]string, len(messages))
	for _, message := range messages {
		result[message.ID] = message.Other
	}
	return result
}

func testSupportedLocales() []servertest.SupportedLocale {
	locales := make([]servertest.SupportedLocale, len(supportedLanguages))
	for index, supported := range supportedLanguages {
		locales[index] = servertest.SupportedLocale{Tag: supported.Tag, Direction: supported.Direction}
	}
	return locales
}
