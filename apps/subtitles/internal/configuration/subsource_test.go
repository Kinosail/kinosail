package configuration_test

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestSubSourceConfigurationRequiresSafePersonalUse(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"KINOSAIL_SUBSOURCE_URL":          "https://api.subsource.net/api/v1",
		"KINOSAIL_SUBSOURCE_API_KEY":      "app-key",
		"KINOSAIL_SUBSOURCE_PERSONAL_USE": "true",
	}
	if _, err := configuration.Load(t.TempDir(), "", lookup(valid)); err != nil {
		t.Fatalf("valid SubSource configuration: %v", err)
	}
	for name, change := range map[string]map[string]string{
		"missing API key":       {"KINOSAIL_SUBSOURCE_API_KEY": ""},
		"missing acceptance":    {"KINOSAIL_SUBSOURCE_PERSONAL_USE": ""},
		"declined personal use": {"KINOSAIL_SUBSOURCE_PERSONAL_USE": "false"},
		"insecure URL":          {"KINOSAIL_SUBSOURCE_URL": "http://api.subsource.net/api/v1"},
		"URL credentials":       {"KINOSAIL_SUBSOURCE_URL": "https://owner@api.subsource.net/api/v1"},
		"URL query":             {"KINOSAIL_SUBSOURCE_URL": "https://api.subsource.net/api/v1?x=1"},
		"spaced key":            {"KINOSAIL_SUBSOURCE_API_KEY": " key"},
		"oversized key":         {"KINOSAIL_SUBSOURCE_API_KEY": strings.Repeat("x", 4097)},
		"control key":           {"KINOSAIL_SUBSOURCE_API_KEY": "key\rvalue"},
	} {
		t.Run(name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			maps.Copy(values, valid)
			maps.Copy(values, change)
			if _, err := configuration.Load(t.TempDir(), "", lookup(values)); err == nil {
				t.Fatal("invalid SubSource configuration was accepted")
			}
		})
	}
}

func TestSubSourceCredentialMutationRejectsControlsWithoutWrite(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := configuration.SetSubSource(directory, "safe-key"); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(directory, "configuration.json"), filepath.Join(directory, "secrets.json")}
	before := make([][]byte, len(paths))
	for index, path := range paths {
		var err error
		if before[index], err = os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := configuration.SetSubSource(directory, "key\tvalue"); err == nil {
		t.Fatal("unsafe SubSource credential mutation was accepted")
	}
	for index, path := range paths {
		after, err := os.ReadFile(path)
		if err != nil || !slices.Equal(after, before[index]) {
			t.Fatalf("rejected mutation changed %s: error=%v", filepath.Base(path), err)
		}
	}
}
