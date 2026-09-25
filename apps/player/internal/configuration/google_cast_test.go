package configuration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/configuration"
)

func TestGoogleCastAppIDValidationBeforePersistence(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	key := "integrations.google_cast.app_id"
	for _, invalid := range []string{"short", "FFFFFFFFF", "DEADBEAG", "DEAD BEE", "DEADBEE\n", strings.Repeat("A", 4096)} {
		if err := configuration.Set(directory, key, invalid); err == nil {
			t.Fatalf("invalid Cast app ID %q was accepted", invalid)
		}
		if _, err := os.Stat(filepath.Join(directory, "configuration.json")); !os.IsNotExist(err) {
			t.Fatalf("invalid Cast app ID wrote configuration: %v", err)
		}
	}
	if _, err := configuration.Load(t.TempDir(), "", func(key string) (string, bool) {
		return "invalid!", key == "KINOSAIL_GOOGLE_CAST_APP_ID"
	}); err == nil {
		t.Fatal("invalid deployment Cast app ID was accepted")
	}
	if err := configuration.Set(directory, key, "deadbeef"); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String(key) != "deadbeef" {
		t.Fatalf("saved Cast app ID = %q, %v", loaded.String(key), err)
	}
	if err := configuration.Set(directory, key, "invalid!"); err == nil {
		t.Fatal("invalid replacement was accepted")
	}
	unchanged, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || unchanged.String(key) != "deadbeef" {
		t.Fatalf("rejected replacement changed the Cast app ID: %q, %v", unchanged.String(key), err)
	}
}
