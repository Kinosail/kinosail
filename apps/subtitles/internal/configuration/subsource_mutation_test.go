package configuration_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-subtitles/internal/configuration"
)

func TestSubSourceMutationPersistsAndDeletesThePair(t *testing.T) { //nolint:cyclop // One lifecycle proves paired persistence, deletion, validation, and secret separation.
	t.Parallel()
	directory := t.TempDir()
	if err := configuration.SetSubSource(directory, "app-key"); err != nil {
		t.Fatal(err)
	}
	loaded, err := configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.subsource.api_key") != "app-key" || !loaded.Bool("integrations.subsource.personal_use") {
		t.Fatalf("stored SubSource pair = key %q, accepted %v, error %v", loaded.String("integrations.subsource.api_key"), loaded.Bool("integrations.subsource.personal_use"), err)
	}
	regular, regularErr := os.ReadFile(filepath.Join(directory, "configuration.json"))
	secrets, secretsErr := os.ReadFile(filepath.Join(directory, "secrets.json"))
	if regularErr != nil || secretsErr != nil || bytes.Contains(regular, []byte("app-key")) || !bytes.Contains(regular, []byte("personal_use")) || !bytes.Contains(secrets, []byte("app-key")) {
		t.Fatalf("stored SubSource files = regular %q, secrets %q, errors %v %v", regular, secrets, regularErr, secretsErr)
	}

	if err = configuration.DeleteSubSource(directory); err != nil {
		t.Fatal(err)
	}
	loaded, err = configuration.Load(directory, "", func(string) (string, bool) { return "", false })
	if err != nil || loaded.String("integrations.subsource.api_key") != "" || loaded.Bool("integrations.subsource.personal_use") {
		t.Fatalf("deleted SubSource pair = key %q, accepted %v, error %v", loaded.String("integrations.subsource.api_key"), loaded.Bool("integrations.subsource.personal_use"), err)
	}
}

func TestSubSourceMutationRejectsInvalidInputWithoutSideEffects(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	if err := configuration.SetSubSource(directory, "original-key"); err != nil {
		t.Fatal(err)
	}
	regularPath := filepath.Join(directory, "configuration.json")
	secretsPath := filepath.Join(directory, "secrets.json")
	regularBefore, _ := os.ReadFile(regularPath)
	secretsBefore, _ := os.ReadFile(secretsPath)
	for name, key := range map[string]string{
		"missing":   "",
		"spaced":    " key",
		"oversized": strings.Repeat("x", 4097),
	} {
		t.Run(name, func(t *testing.T) {
			if err := configuration.SetSubSource(directory, key); err == nil {
				t.Fatal("invalid SubSource key was accepted")
			}
			regular, _ := os.ReadFile(regularPath)
			secrets, _ := os.ReadFile(secretsPath)
			if !bytes.Equal(regular, regularBefore) || !bytes.Equal(secrets, secretsBefore) {
				t.Fatalf("rejected mutation changed files to regular %q, secrets %q", regular, secrets)
			}
		})
	}
}

func TestSubSourceMutationRejectsUnavailableAndInvalidStorage(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.SetSubSource(file, "app-key"); err == nil {
		t.Fatal("SubSource set accepted a regular file as its data directory")
	}
	if err := configuration.DeleteSubSource(file); err == nil {
		t.Fatal("SubSource delete accepted a regular file as its data directory")
	}

	directory := t.TempDir()
	invalid := []byte(`{"unknown":"value"}`)
	if err := os.WriteFile(filepath.Join(directory, "configuration.json"), invalid, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := configuration.SetSubSource(directory, "app-key"); err == nil {
		t.Fatal("SubSource set accepted invalid stored state")
	}
	if err := configuration.DeleteSubSource(directory); err == nil {
		t.Fatal("SubSource delete accepted invalid stored state")
	}
	stored, err := os.ReadFile(filepath.Join(directory, "configuration.json"))
	if err != nil || !bytes.Equal(stored, invalid) {
		t.Fatalf("rejected stored state changed to %q, error %v", stored, err)
	}
}
