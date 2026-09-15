package configuration_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestOpenSubtitlesCredentialChangesAtomically(t *testing.T) {
	t.Parallel()
	directory := configuredOpenSubtitles(t)
	if err := configuration.DeleteOpenSubtitles(directory); err != nil {
		t.Fatal(err)
	}
	cleared, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || cleared.String("integrations.opensubtitles.api_key") != "" || cleared.String("integrations.opensubtitles.username") != "" || cleared.String("integrations.opensubtitles.password") != "" {
		t.Fatalf("deleted credential remains configured: error=%v", err)
	}
}

func TestOpenSubtitlesCredentialRejectionPreservesState(t *testing.T) {
	t.Parallel()
	directory := configuredOpenSubtitles(t)
	if err := configuration.SetOpenSubtitles(directory, "app-key", "owner", "password"); err != nil {
		t.Fatal(err)
	}
	before, readErr := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err := configuration.SetOpenSubtitles(directory, "new-key", strings.Repeat("x", (16<<10)+1), "new-password"); err == nil {
		t.Fatal("oversized OpenSubtitles credential was accepted")
	}
	after, readErr := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if readErr != nil || string(after) != string(before) {
		t.Fatalf("rejected credential changed state: error=%v", readErr)
	}
	if err := configuration.Set(directory, "integrations.opensubtitles.api_key", "partial-key"); err == nil {
		t.Fatal("individual OpenSubtitles setting was accepted")
	}
}

func configuredOpenSubtitles(t *testing.T) string {
	t.Helper()
	directory := t.TempDir()
	if err := configuration.SetOpenSubtitles(directory, "app-key", "owner", "password"); err != nil {
		t.Fatal(err)
	}
	configured, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || configured.String("integrations.opensubtitles.api_key") != "app-key" || configured.String("integrations.opensubtitles.username") != "owner" || configured.String("integrations.opensubtitles.password") != "password" {
		t.Fatalf("saved credential = %q/%q/%q, error=%v", configured.String("integrations.opensubtitles.api_key"), configured.String("integrations.opensubtitles.username"), configured.String("integrations.opensubtitles.password"), err)
	}
	return directory
}

func TestOpenSubtitlesMutationPropagatesStoredStateErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("value"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.SetOpenSubtitles(file, "app-key", "owner", "password"); err == nil {
		t.Fatal("OpenSubtitles set accepted a file as its data directory")
	}
	if err := configuration.DeleteOpenSubtitles(file); err == nil {
		t.Fatal("OpenSubtitles delete accepted a file as its data directory")
	}

	partialSubSource := t.TempDir()
	if err := os.WriteFile(filepath.Join(partialSubSource, "configuration.json"), []byte(`{"integrations.subsource.personal_use":"true"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.SetOpenSubtitles(partialSubSource, "app-key", "owner", "password"); err == nil {
		t.Fatal("OpenSubtitles set accepted invalid stored configuration")
	}

	partialOIDC := t.TempDir()
	if err := os.WriteFile(filepath.Join(partialOIDC, "configuration.json"), []byte(`{"integrations.oidc.issuer":"https://identity.example"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.DeleteOpenSubtitles(partialOIDC); err == nil {
		t.Fatal("OpenSubtitles delete accepted invalid stored configuration")
	}
}
