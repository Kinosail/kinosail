package configuration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
)

func TestLoadIgnoresRetiredSubtitleProviderConfiguration(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "configuration.json"), []byte(`{"integrations.subdl.url":"https://api.subdl.com/api/v1"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "secrets.json"), []byte(`{"integrations.subdl.api_key":"retired-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(directory, "kinosail.yaml")
	if err := os.WriteFile(yamlPath, []byte("version: 1\nintegrations:\n  subdl:\n    url: https://api.subdl.com/api/v1\n    api_key_file: retired\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, yamlPath, func(string) (string, bool) { return "", false })
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range loaded.Fields() {
		if strings.HasPrefix(field.Key, "integrations.subdl.") {
			t.Fatalf("retired subtitle provider remains configured: %#v", field)
		}
	}
}
