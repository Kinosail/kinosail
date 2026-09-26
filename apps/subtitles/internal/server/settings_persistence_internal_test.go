package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
)

func TestSettingsRejectInvalidPersistedSubtitleLanguagesWithoutSave(t *testing.T) {
	t.Parallel()
	twentyOne, _ := json.Marshal(append([]string{"en", "fr", "de", "es", "it", "nl", "pl", "pt", "ru", "uk", "tr", "ar", "fa", "he", "hi", "bn", "ur", "id", "ms", "vi"}, "th"))
	cases := map[string]string{
		"whole document null":  `null`,
		"unknown field":        `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["en"],"unexpected":true}`,
		"duplicate field":      `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["en"],"SubtitleLanguages":["fr"]}`,
		"wrong type":           `{"name":"Kinosail","libraries":["."],"subtitleLanguages":"en"}`,
		"picker wrong type":    `{"name":"Kinosail","libraries":["."],"subtitlePickerLimited":"on"}`,
		"picker null":          `{"name":"Kinosail","libraries":["."],"subtitlePickerLimited":null}`,
		"wrong patron type":    `{"name":"Kinosail","libraries":["."],"supporter":{"patronOrder":[]}}`,
		"unknown patron field": `{"name":"Kinosail","libraries":["."],"supporter":{"patronOrder":{"unexpected":true}}}`,
		"malformed patron":     `{"name":"Kinosail","libraries":["."],"supporter":{"patronOrder":{`,
		"trailing document":    `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["en"]} {}`,
		"null list":            `{"name":"Kinosail","libraries":["."],"subtitleLanguages":null}`,
		"null singular":        `{"name":"Kinosail","libraries":["."],"subtitleLanguage":null,"subtitleLanguages":["en"]}`,
		"empty list":           `{"name":"Kinosail","libraries":["."],"subtitleLanguages":[]}`,
		"too many":             `{"name":"Kinosail","libraries":["."],"subtitleLanguages":` + string(twentyOne) + `}`,
		"canonical duplicate":  `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["pt-br","pt-BR"]}`,
		"unknown language":     `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["not-a-language"]}`,
		"provider spelling":    `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["ea"]}`,
		"base and variant":     `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["pt","pt-BR"]}`,
		"primary conflict":     `{"name":"Kinosail","libraries":["."],"subtitleLanguage":"fr","subtitleLanguages":["en","fr"]}`,
		"oversized document":   `{"name":"` + strings.Repeat("x", (1<<20)+1) + `","libraries":["."],"subtitleLanguages":["en"]}`,
		"oversized patron":     `{"name":"Kinosail","libraries":["."],"supporter":{"patronOrder":{"certificate":"` + strings.Repeat("x", (1<<20)+1) + `"}}}`,
	}
	for name, document := range cases {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "settings.json")
			original := []byte(document)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			store := newSettingsStore("", directory, "", nil)
			if store.err == nil {
				t.Fatal("invalid persisted settings were accepted")
			}
			current, err := os.ReadFile(path)
			if err != nil || !slices.Equal(current, original) {
				t.Fatalf("invalid startup changed settings: error=%v changed=%v", err, !slices.Equal(current, original))
			}
		})
	}
}

func TestServerRejectsInvalidLegacySubtitleLanguagesBeforeDatabaseMigration(t *testing.T) { //nolint:gocognit // Every rejected legacy input proves migration has no side effects.
	t.Parallel()
	twentyOne, _ := json.Marshal(append([]string{"en", "fr", "de", "es", "it", "nl", "pl", "pt", "ru", "uk", "tr", "ar", "fa", "he", "hi", "bn", "ur", "id", "ms", "vi"}, "th"))
	for name, document := range map[string]string{
		"empty":     `{"name":"Kinosail","libraries":["."],"subtitleLanguages":[]}`,
		"too many":  `{"name":"Kinosail","libraries":["."],"subtitleLanguages":` + string(twentyOne) + `}`,
		"duplicate": `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["pt-br","pt-BR"]}`,
		"overlap":   `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["pt","pt-BR"]}`,
		"unknown":   `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["not-a-language"]}`,
		"provider":  `{"name":"Kinosail","libraries":["."],"subtitleLanguages":["zh_bg"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			settingsPath := filepath.Join(directory, "settings.json")
			original := []byte(document)
			if err := os.WriteFile(settingsPath, original, 0o600); err != nil {
				t.Fatal(err)
			}
			response := httptest.NewRecorder()
			New(Config{DataDir: directory}).ServeHTTP(response, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("invalid startup = %d, body = %q", response.Code, response.Body.String())
			}
			current, err := os.ReadFile(settingsPath)
			if err != nil || !slices.Equal(current, original) {
				t.Fatalf("migration changed settings: error=%v changed=%v", err, !slices.Equal(current, original))
			}
			for _, name := range []string{database.Filename, database.Filename + "-wal", database.Filename + "-shm"} {
				if _, err = os.Lstat(filepath.Join(directory, name)); !os.IsNotExist(err) {
					t.Fatalf("migration created %s after rejection: %v", name, err)
				}
			}
		})
	}
}

func TestLoadStateRejectsNonRegularAndOversizedFiles(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name  string
		build func(*testing.T, string) string
	}{
		{"symlink", func(t *testing.T, directory string) string {
			t.Helper()
			target := filepath.Join(directory, "target.json")
			if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, "state.json")
			if err := os.Symlink(target, path); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{"oversized", func(t *testing.T, directory string) string {
			t.Helper()
			path := filepath.Join(directory, "state.json")
			if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.Truncate(path, database.DocumentSizeLimit+1); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := test.build(t, t.TempDir())
			var target map[string]any
			if found, err := loadState(nil, path, &target); err == nil || found {
				t.Fatalf("unsafe state = found:%v error:%v", found, err)
			}
		})
	}
}
