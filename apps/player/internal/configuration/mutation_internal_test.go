package configuration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/configurationcore"
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
	for name, mutate := range map[string]func() error{
		"set":    func() error { return Set(file, "server.name", "Broken") },
		"delete": func() error { return Delete(file, "server.name") },
		"set SCIM": func() error {
			return SetSCIM(file, "scim-test-token-012345678901234567890", time.Now().UTC().Add(time.Hour).Format(time.RFC3339))
		},
		"delete SCIM": func() error { return DeleteSCIM(file) },
	} {
		if err := mutate(); err == nil {
			t.Fatalf("%s accepted a regular file as its data directory", name)
		}
	}
	if err := persistStoredConfiguration(file, nil, nil, true, true); err == nil {
		t.Fatal("paired persistence accepted a regular file as its data directory")
	}
	if err := validateStoredConfiguration(map[string]string{"unknown": "value"}, nil); err == nil {
		t.Fatal("unknown regular configuration key was accepted")
	}
	if err := validateStoredConfiguration(map[string]string{scimTokenKey: "value"}, nil); err == nil {
		t.Fatal("secret configuration key in regular state was accepted")
	}
	if err := validateStoredConfiguration(nil, map[string]string{scimExpirationKey: "value"}); err == nil {
		t.Fatal("regular configuration key in secret state was accepted")
	}
}

func TestConfigurationTransactionRecoversPairedFiles(t *testing.T) {
	directory := t.TempDir()
	regular := []byte(`{"server.name":"Recovered"}`)
	secrets := []byte(`{"backup.key":"recovered"}`)
	if err := storedConfigurationTransaction.Write(directory, regular, secrets); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readStoredConfiguration(directory); err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string][]byte{configurationFile: regular, secretsFile: secrets} {
		data, err := os.ReadFile(filepath.Join(directory, path))
		if err != nil || string(data) != string(want) {
			t.Fatalf("recovered %s = %q err=%v", path, data, err)
		}
	}
	if _, err := os.Stat(filepath.Join(directory, configurationcore.TransactionFilename)); !os.IsNotExist(err) {
		t.Fatalf("configuration transaction remains: %v", err)
	}
	var decoded map[string]string
	if err := json.Unmarshal(regular, &decoded); err != nil || decoded["server.name"] != "Recovered" {
		t.Fatalf("recovered regular JSON = %#v err=%v", decoded, err)
	}
}

func TestConfigurationTransactionRejectsInvalidRecoveryState(t *testing.T) { //nolint:cyclop // Recovery failure paths share one direct matrix.
	directory := t.TempDir()
	marker := filepath.Join(directory, configurationcore.TransactionFilename)
	for _, data := range [][]byte{[]byte("{"), []byte(`{"regular":"e30="}`)} {
		if err := os.WriteFile(marker, data, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readStoredConfiguration(directory); err == nil {
			t.Fatalf("invalid transaction %q was accepted", data)
		}
	}
	file := filepath.Join(directory, "file")
	if err := os.WriteFile(file, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := storedConfigurationTransaction.Write(filepath.Join(file, "child"), []byte("{}"), []byte("{}")); err == nil {
		t.Fatal("transaction under a regular file was accepted")
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(marker, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(marker, "entry"), []byte("entry"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := storedConfigurationTransaction.Recover(directory); err == nil {
		t.Fatal("transaction directory was read as a transaction")
	}
	if err := storedConfigurationTransaction.Remove(directory); err == nil {
		t.Fatal("transaction directory was removed as a file")
	}
	if err := storedConfigurationTransaction.Remove(filepath.Join(directory, "missing")); err != nil {
		t.Fatalf("missing transaction removal failed: %v", err)
	}
}

func TestConfigurationTransactionRejectsSemanticallyInvalidRecoveryWithoutSideEffects(t *testing.T) { //nolint:cyclop,gocognit // Each invalid journal state shares the same no-side-effect assertion.
	for name, transaction := range map[string][2][]byte{
		"unknown key": {
			[]byte(`{"unknown":"value"}`),
			[]byte(`{}`),
		},
		"secret in regular state": {
			[]byte(`{"integrations.scim.token":"scim-test-token-012345678901234567890"}`),
			[]byte(`{}`),
		},
		"invalid expiration": {
			[]byte(`{"integrations.scim.token_expires_at":"not-a-time"}`),
			[]byte(`{}`),
		},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			regularPath := filepath.Join(directory, configurationFile)
			secretsPath := filepath.Join(directory, secretsFile)
			regularBefore := []byte(`{"server.name":"Before"}`)
			secretsBefore := []byte(`{"backup.key":"old"}`)
			if err := os.WriteFile(regularPath, regularBefore, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(secretsPath, secretsBefore, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := storedConfigurationTransaction.Write(directory, transaction[0], transaction[1]); err != nil {
				t.Fatal(err)
			}
			if _, _, err := readStoredConfiguration(directory); err == nil {
				t.Fatal("semantically invalid transaction was accepted")
			}
			for path, want := range map[string][]byte{regularPath: regularBefore, secretsPath: secretsBefore} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != string(want) {
					t.Fatalf("recovery changed %s to %q, err=%v", path, got, err)
				}
			}
			if _, err := os.Stat(filepath.Join(directory, configurationcore.TransactionFilename)); err != nil {
				t.Fatalf("invalid transaction marker was removed: %v", err)
			}
		})
	}
}

func TestConfigurationMutationRejectsNullPersistedMaps(t *testing.T) {
	directory := t.TempDir()
	regularBefore := []byte(`{"server.name":"Before"}`)
	secretsBefore := []byte("null")
	if err := os.WriteFile(filepath.Join(directory, configurationFile), regularBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, secretsFile), secretsBefore, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SetSCIM(directory, "scim-test-token-012345678901234567890", time.Now().UTC().Add(time.Hour).Format(time.RFC3339)); err == nil {
		t.Fatal("SCIM mutation accepted a null persisted map")
	}
	for path, want := range map[string][]byte{
		filepath.Join(directory, configurationFile): regularBefore,
		filepath.Join(directory, secretsFile):       secretsBefore,
	} {
		got, readErr := os.ReadFile(path)
		if readErr != nil || string(got) != string(want) {
			t.Fatalf("rejected SCIM mutation changed %s to %q, err=%v", path, got, readErr)
		}
	}
}

func TestConfigurationTransactionRecoverySurvivesTargetFailure(t *testing.T) {
	for _, target := range []string{configurationFile, secretsFile} {
		directory := t.TempDir()
		if err := storedConfigurationTransaction.Write(directory, []byte(`{}`), []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(directory, target), 0o700); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readStoredConfiguration(directory); err == nil {
			t.Fatalf("recovery with %s target directory succeeded", target)
		}
		if err := os.Remove(filepath.Join(directory, target)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := readStoredConfiguration(directory); err != nil {
			t.Fatalf("recovery after %s target repair failed: %v", target, err)
		}
	}
}
