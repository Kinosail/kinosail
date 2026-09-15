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

func TestSubDLConfigurationRejectsUnsafeBoundaries(t *testing.T) {
	t.Parallel()
	valid := map[string]string{
		"KINOSAIL_SUBDL_URL":     "https://api.subdl.com/api/v1",
		"KINOSAIL_SUBDL_API_KEY": "app-key",
	}
	if _, err := configuration.Load(t.TempDir(), "", lookup(valid)); err != nil {
		t.Fatalf("valid SubDL configuration: %v", err)
	}
	for name, change := range map[string]map[string]string{
		"insecure URL":    {"KINOSAIL_SUBDL_URL": "http://api.subdl.com/api/v1"},
		"URL credentials": {"KINOSAIL_SUBDL_URL": "https://owner@api.subdl.com/api/v1"},
		"URL query":       {"KINOSAIL_SUBDL_URL": "https://api.subdl.com/api/v1?x=1"},
		"oversized URL":   {"KINOSAIL_SUBDL_URL": "https://api.subdl.com/" + strings.Repeat("x", 2049)},
		"spaced key":      {"KINOSAIL_SUBDL_API_KEY": " key"},
		"oversized key":   {"KINOSAIL_SUBDL_API_KEY": strings.Repeat("x", 4097)},
		"control key":     {"KINOSAIL_SUBDL_API_KEY": "key\x7fvalue"},
	} {
		t.Run(name, func(t *testing.T) {
			values := make(map[string]string, len(valid))
			maps.Copy(values, valid)
			maps.Copy(values, change)
			if _, err := configuration.Load(t.TempDir(), "", lookup(values)); err == nil {
				t.Fatal("invalid SubDL configuration was accepted")
			}
		})
	}
}

func TestProviderCredentialMutationRejectsControlsWithoutWrite(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := configuration.Set(directory, "integrations.subdl.api_key", "safe-key"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "secrets.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = configuration.Set(directory, "integrations.subdl.api_key", "key\nvalue"); err == nil {
		t.Fatal("unsafe provider credential mutation was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil || !slices.Equal(after, before) {
		t.Fatalf("rejected mutation changed secrets: error=%v", err)
	}
}
