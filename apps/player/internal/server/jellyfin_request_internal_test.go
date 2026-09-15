package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsRejectInvalidPersistedJellyfinIdentifier(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	settings := `{"name":"Kinosail","libraries":["."],"jellyfinCompatibility":true,"jellyfinId":"bad\nid","navigation":["home"]}`
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(settings), 0o600); err != nil {
		t.Fatal(err)
	}
	if store := newSettingsStore("", directory, "", nil); store.err == nil {
		t.Fatal("invalid Jellyfin server identifier was accepted")
	}
}
