package server

import (
	"encoding/json"
	"testing"

	"github.com/MikeO7/kinosail/packages/localization"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestLocaleCatalogsAreComplete(t *testing.T) {
	t.Parallel()
	servertest.LocaleCatalogsAreComplete(t, testSupportedLocales(), readTestCatalog)
}

func TestNewInteractiveCopyHasTranslations(t *testing.T) {
	t.Parallel()
	english := readTestCatalog(t, "en")
	messages := []string{"Generated API key", "Copy API key", "Copied.", "Select Copy in your browser.", "Set up trusted HTTPS", "Review trusted HTTPS setup", "Offline copy", "Verified file stored on this device.", "Ready offline on this device", "Offline copy is unavailable.", "Offline playback needs an active service worker", "Could not remove local offline data.", "stored on this device", "This device does not have enough storage for this download", "This browser cannot store offline media safely", "The download could not be verified", "The download is no longer available", "No verified downloads are ready for this Viewer Profile.", "Open Kinosail online and choose a Viewer Profile first.", "Playback method", "Open playback settings."}
	for _, supported := range supportedLanguages {
		tag := supported.Tag
		if !localization.IsTranslated(tag) {
			continue
		}
		if tag == "en" {
			continue
		}
		catalog := readTestCatalog(t, tag)
		for _, message := range messages {
			if catalog[message] == english[message] {
				t.Errorf("%s leaves %q in English", tag, message)
			}
		}
	}
}

func TestSupportedLanguageCoverageIncludesJellyfinLocaleSet(t *testing.T) {
	t.Parallel()
	servertest.SupportedLanguageCoverageIncludesJellyfinLocaleSet(t, testSupportedLocales())
}

func TestTranslatedCatalogsLocalizeAuthenticationCode(t *testing.T) {
	t.Parallel()
	englishCatalog := readTestCatalog(t, "en")
	for _, message := range []string{"Authentication code", "Authentication or recovery code"} {
		english := englishCatalog[message]
		if english == "" {
			t.Fatalf("English catalog is missing %q", message)
		}
		for _, supported := range supportedLanguages {
			tag := supported.Tag
			if !localization.IsTranslated(tag) {
				continue
			}
			if tag == "en" {
				continue
			}
			translated := readTestCatalog(t, tag)[message]
			if translated == "" || translated == english {
				t.Errorf("%s catalog does not translate %q", tag, message)
			}
		}
	}
}

func readTestCatalog(t *testing.T, tag string) map[string]string {
	t.Helper()
	data, err := localeCatalogs.ReadFile("locales/active." + tag + ".json")
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
