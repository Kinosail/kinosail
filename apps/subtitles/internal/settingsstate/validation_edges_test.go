package settingsstate

import (
	"encoding/json"
	"testing"
)

func TestValidationRejectsIncompleteAndInvalidLanguageDocuments(t *testing.T) {
	t.Parallel()
	for _, data := range []string{
		"{", "{]", `{"name":"Home"`, `{"":true}`,
		`{"subtitleLanguage":"unknown"}`,
	} {
		if err := Validate([]byte(data)); err == nil {
			t.Errorf("invalid document accepted: %q", data)
		}
	}
	if err := Validate([]byte(`{"subtitleLanguages":["en"]}`)); err != nil {
		t.Fatalf("valid document rejected: %v", err)
	}
}

func TestLanguageParserRejectsWrongTypesBeforeReturningPreferences(t *testing.T) {
	t.Parallel()
	for _, fields := range []map[string]json.RawMessage{
		{"subtitlelanguage": json.RawMessage("42")},
		{"subtitlelanguages": json.RawMessage(`"en"`)},
	} {
		preferences, err := parseLanguages(fields)
		if err == nil || preferences.Primary != "" || preferences.Languages != nil {
			t.Fatalf("invalid fields returned preferences: %#v %v", preferences, err)
		}
	}
}
