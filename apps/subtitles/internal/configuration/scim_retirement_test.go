package configuration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestLoadIgnoresRetiredSCIMConfiguration(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "configuration.json"), []byte(`{"integrations.scim.token_expires_at":"2030-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "secrets.json"), []byte(`{"integrations.scim.token":"retired-secret-retired-secret-retired"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(directory, "kinosail.yaml")
	if err := os.WriteFile(yamlPath, []byte("version: 1\nintegrations:\n  scim:\n    token_file: retired\n    token_expires_at: 2030-01-01T00:00:00Z\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, yamlPath, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range loaded.Fields() {
		if strings.HasPrefix(field.Key, "integrations.scim.") {
			t.Fatalf("retired SCIM setting remains configured: %#v", field)
		}
	}
	if err := configuration.Set(directory, "server.name", "Subtitles"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(directory, "configuration.json"))
	if err != nil || strings.Contains(string(data), "integrations.scim.") {
		t.Fatalf("configuration.json after a save = %s, %v", data, err)
	}
}
