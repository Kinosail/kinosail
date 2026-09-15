package configuration

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigurationMutationHelpersRejectInvalidBoundaries(t *testing.T) {
	directory := t.TempDir()
	if err := persistStoredConfiguration(directory, nil, nil, false, false); err != nil {
		t.Fatalf("no-op persistence = %v", err)
	}
	file := filepath.Join(directory, "not-a-directory")
	if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readStoredConfiguration(file); err == nil {
		t.Fatal("file path was accepted as a configuration directory")
	}
	if err := Set(file, "subtitles.language", "es"); err == nil {
		t.Fatal("subtitle mutation accepted a regular file as its data directory")
	}
	if err := persistStoredConfiguration(file, nil, nil, true, true); err == nil {
		t.Fatal("paired persistence accepted a regular file as its data directory")
	}
}

func TestSubtitleConfigurationTransactionUsesSharedRecovery(t *testing.T) {
	directory := t.TempDir()
	if err := storedConfigurationTransaction.Write(directory, []byte(`{"subtitles.language":"es"}`), []byte(`{"backup.key":"recovered"}`)); err != nil {
		t.Fatal(err)
	}
	regular, secrets, err := readStoredConfiguration(directory)
	if err != nil {
		t.Fatal(err)
	}
	if regular["subtitles.language"] != "es" || secrets["backup.key"] != "recovered" {
		t.Fatalf("recovered configuration = %#v, %#v", regular, secrets)
	}
}
