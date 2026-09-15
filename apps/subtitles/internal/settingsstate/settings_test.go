package settingsstate

import (
	"slices"
	"strings"
	"testing"
)

func TestParseCanonicalizesLegacyAndOrderedSubtitleLanguages(t *testing.T) {
	for name, test := range map[string]struct {
		data    string
		primary string
		wanted  []string
	}{
		"default":  {`{"name":"Home"}`, "en", []string{"en"}},
		"singular": {`{"subtitleLanguage":"pt-br"}`, "pt-BR", []string{"pt-BR"}},
		"ordered":  {`{"subtitleLanguage":"fr","subtitleLanguages":["fr","zh-cn","es-419"],"subtitlePreference":"sdh"}`, "fr", []string{"fr", "zh-Hans", "es-419"}},
	} {
		t.Run(name, func(t *testing.T) {
			preferences, err := Parse([]byte(test.data))
			if err != nil || preferences.Primary != test.primary || !slices.Equal(preferences.Languages, test.wanted) {
				t.Fatalf("Parse() = %#v, %v", preferences, err)
			}
		})
	}
}

func TestParseAcceptsIndependentSupporterGrantFamilies(t *testing.T) {
	data := []byte(`{"supporter":{"installationKey":"installation","patronLevel":8,"livingLevel":10,"patronOrder":{"activationId":"patron","certificate":"certificate","signature":"signature","publicKey":"public","keyHash":"hash"},"livingStandard":{"activationId":"living","certificate":"certificate","signature":"signature","publicKey":"public","keyHash":"hash"}}}`)
	preferences, err := Parse(data)
	if err != nil || preferences.Primary != "en" || !slices.Equal(preferences.Languages, []string{"en"}) {
		t.Fatalf("Parse() = %#v, %v", preferences, err)
	}
}

func TestParseRejectsInvalidSettingsLanguageState(t *testing.T) {
	twentyOne := `["en","fr","de","es","it","nl","pl","pt","ru","uk","tr","ar","fa","he","hi","bn","ur","id","ms","vi","th"]`
	for name, data := range map[string]string{
		"not object":             `[]`,
		"unknown field":          `{"unexpected":true}`,
		"duplicate field":        `{"subtitleLanguages":["en"],"SubtitleLanguages":["fr"]}`,
		"trailing document":      `{} {}`,
		"null singular":          `{"subtitleLanguage":null}`,
		"null list":              `{"subtitleLanguages":null}`,
		"null preference":        `{"subtitlePreference":null}`,
		"unknown preference":     `{"subtitlePreference":"unknown"}`,
		"uppercase preference":   `{"subtitlePreference":"SDH"}`,
		"wrong singular type":    `{"subtitleLanguage":42}`,
		"wrong list type":        `{"subtitleLanguages":"en"}`,
		"wrong name type":        `{"name":42}`,
		"wrong libraries type":   `{"libraries":"Movies"}`,
		"wrong MFA type":         `{"requireMfa":"yes"}`,
		"wrong supporter type":   `{"supporter":[]}`,
		"negative badge level":   `{"supporter":{"patronLevel":-1}}`,
		"oversized badge level":  `{"supporter":{"livingLevel":11}}`,
		"wrong patron type":      `{"supporter":{"patronOrder":[]}}`,
		"wrong living type":      `{"supporter":{"livingStandard":"active"}}`,
		"unknown patron field":   `{"supporter":{"patronOrder":{"unexpected":true}}}`,
		"unknown living field":   `{"supporter":{"livingStandard":{"unexpected":true}}}`,
		"wrong grant field type": `{"supporter":{"patronOrder":{"certificate":42}}}`,
		"malformed grant":        `{"supporter":{"patronOrder":{`,
		"wrong navigation type":  `{"navigation":{}}`,
		"empty list":             `{"subtitleLanguages":[]}`,
		"too many":               `{"subtitleLanguages":` + twentyOne + `}`,
		"canonical duplicate":    `{"subtitleLanguages":["pt-br","pt-BR"]}`,
		"overlap":                `{"subtitleLanguages":["pt","pt-BR"]}`,
		"unknown language":       `{"subtitleLanguages":["not-a-language"]}`,
		"primary conflict":       `{"subtitleLanguage":"fr","subtitleLanguages":["en","fr"]}`,
		"invalid UTF-8":          string([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}),
		"oversized":              `{"name":"` + strings.Repeat("x", MaximumDocumentSize) + `"}`,
		"oversized grant":        `{"supporter":{"patronOrder":{"certificate":"` + strings.Repeat("x", MaximumDocumentSize) + `"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(data)); err == nil {
				t.Fatal("invalid settings state was accepted")
			}
		})
	}
}

func TestValidateSubtitleLanguagesRejectsInvalidListsAtomically(t *testing.T) {
	for _, languages := range [][]string{nil, {"not-a-language"}, {"pt-br", "pt-BR"}, {"pt", "pt-BR"}} {
		if canonical, err := ValidateSubtitleLanguages(languages); err == nil || canonical != nil {
			t.Fatalf("ValidateSubtitleLanguages(%q) = %q, %v", languages, canonical, err)
		}
	}
	canonical, err := ValidateSubtitleLanguages([]string{"EN", "pt-br", "zh-cn"})
	if err != nil || !slices.Equal(canonical, []string{"en", "pt-BR", "zh-Hans"}) {
		t.Fatalf("canonical languages = %q, %v", canonical, err)
	}
}
