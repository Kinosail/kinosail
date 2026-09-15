package commandtest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ConfigurationSnapshot exposes the values needed by the shared CLI contract.
type ConfigurationSnapshot interface {
	String(string) string
}

// ConfigurationCommandSources verifies config validation and source summaries.
func ConfigurationCommandSources[Snapshot any](t *testing.T, set func(string, string, string) error, load func(string, string, func(string) (string, bool)) (Snapshot, error), command func([]string, io.Writer, func(string) (Snapshot, error)) (bool, error)) { //nolint:cyclop // This shared contract keeps every CLI validation boundary in one scenario.
	t.Parallel()
	directory := t.TempDir()
	file := filepath.Join(directory, "kinosail.yaml")
	if err := os.WriteFile(file, []byte("version: 1\nserver:\n  name: File Home\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := set(directory, "backup.retention", "12"); err != nil {
		t.Fatal(err)
	}
	configured := func(path string) (Snapshot, error) {
		return load(directory, path, func(name string) (string, bool) {
			return "warn", name == "KINOSAIL_LOG_LEVEL"
		})
	}
	var output bytes.Buffer
	handled, err := command([]string{"config", "validate", file}, &output, configured)
	if !handled || err != nil || !strings.Contains(output.String(), "Configuration is valid") || !strings.Contains(output.String(), "1 file") || !strings.Contains(output.String(), "1 UI") || !strings.Contains(output.String(), "1 environment") {
		t.Fatalf("config validate = %v, %v, %q", handled, err, output.String())
	}
	if handled, err = command([]string{"config", "validate", filepath.Join(directory, "missing.yaml")}, io.Discard, configured); !handled || err == nil {
		t.Fatalf("missing config = %v, %v", handled, err)
	}
	if handled, err = command([]string{"config", "unknown"}, io.Discard, configured); !handled || err == nil {
		t.Fatalf("unknown config command = %v, %v", handled, err)
	}
}

func LoadSnapshot[Snapshot any](t *testing.T, load func(string, string, func(string) (string, bool)) (Snapshot, error), values map[string]string) Snapshot {
	t.Helper()
	configured, err := load(t.TempDir(), "", func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	return configured
}

// ConfigurationEdges verifies defaults, explicit origins, and remote configuration.
func ConfigurationEdges[Snapshot any](t *testing.T, load func(string, string, func(string) (string, bool)) (Snapshot, error), authURL func(Snapshot) string, wireGuard func(Snapshot) (string, string), port string) {
	var zero Snapshot
	if got := authURL(zero); got != "http://localhost:"+port {
		t.Fatalf("zero auth URL = %q", got)
	}
	explicit := LoadSnapshot(t, load, map[string]string{"KINOSAIL_AUTH_URL": "https://player.example"})
	if got := authURL(explicit); got != "https://player.example" {
		t.Fatalf("explicit auth URL = %q", got)
	}
	configured := LoadSnapshot(t, load, map[string]string{
		"KINOSAIL_REMOTE_MODE": "wireguard", "KINOSAIL_DUCKDNS_DOMAIN": "family-media",
		"KINOSAIL_DUCKDNS_TOKEN": strings.Repeat("a", 32), "KINOSAIL_WIREGUARD_DIR": "/wireguard",
	})
	directory, endpoint := wireGuard(configured)
	if directory != "/wireguard" || endpoint != "family-media.duckdns.org:51820" {
		t.Fatalf("WireGuard directory=%q endpoint=%q", directory, endpoint)
	}
}

func DefaultConfigurationFile[Snapshot ConfigurationSnapshot](t *testing.T, load func(string) (Snapshot, error)) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "kinosail.yaml"), []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KINOSAIL_DATA_DIR", directory)
	t.Setenv("KINOSAIL_CONFIG_FILE", "")
	configured, err := load("")
	if err != nil || configured.String("paths.data") != directory {
		t.Fatalf("default data directory=%q error=%v", configured.String("paths.data"), err)
	}
}
