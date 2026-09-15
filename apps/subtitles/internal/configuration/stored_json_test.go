package configuration

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestStoredJSONRejectsMalformedDuplicateTrailingAndOversizedDocuments(t *testing.T) {
	for name, document := range invalidStoredJSONDocuments(t) {
		t.Run(name, func(t *testing.T) {
			if _, err := parseJSON(document, configurationFile); err == nil {
				t.Fatal("invalid stored JSON was accepted")
			}
		})
	}
	valid, err := parseJSON([]byte("{\"server.name\":\"Living Room\"}\n"), configurationFile)
	if err != nil || valid["server.name"] != "Living Room" {
		t.Fatalf("valid stored JSON = %#v, error %v", valid, err)
	}
}

func TestInvalidStoredJSONCausesNoConfigurationMutation(t *testing.T) { //nolint:gocognit // One matrix proves identical no-side-effect behavior for every invalid stored document.
	for name, document := range invalidStoredJSONDocuments(t) {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, configurationFile)
			if err := os.WriteFile(path, document, 0o600); err != nil {
				t.Fatal(err)
			}
			for operation, mutate := range map[string]func() error{
				"set":    func() error { return Set(directory, "server.name", "Changed") },
				"delete": func() error { return Delete(directory, "server.name") },
			} {
				if err := mutate(); err == nil {
					t.Fatalf("%s accepted invalid stored JSON", operation)
				}
				stored, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(stored, document) {
					t.Fatalf("%s changed invalid stored JSON", operation)
				}
			}
		})
	}
}

func TestConfigurationMutationRejectsExternalStoredFileSymlink(t *testing.T) {
	directory := t.TempDir()
	externalPath := filepath.Join(t.TempDir(), "external.json")
	external := []byte(`{"server.name":"External"}`)
	if err := os.WriteFile(externalPath, external, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalPath, filepath.Join(directory, configurationFile)); err != nil {
		t.Fatal(err)
	}
	if err := Set(directory, "server.name", "Changed"); err == nil {
		t.Fatal("configuration mutation followed an external stored-file symlink")
	}
	got, err := os.ReadFile(externalPath)
	if err != nil || !bytes.Equal(got, external) {
		t.Fatalf("external stored file changed to %q, err=%v", got, err)
	}
}

func invalidStoredJSONDocuments(t *testing.T) map[string][]byte {
	t.Helper()
	tooMany := make(map[string]string, len(specs)+1)
	for index := 0; index <= len(specs); index++ {
		tooMany[strconv.Itoa(index)] = "value"
	}
	tooManyDocument, err := json.Marshal(tooMany)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{
		"empty":         {},
		"array":         []byte(`[]`),
		"null":          []byte(`null`),
		"malformed":     []byte(`{"server.name":`),
		"incomplete":    []byte(`{"server.name":"First"`),
		"wrong close":   []byte(`{"server.name":"First"]`),
		"duplicate":     []byte(`{"server.name":"First","server.name":"Second"}`),
		"non-string":    []byte(`{"server.name":1}`),
		"invalid UTF-8": []byte("{\"server.name\":\"\xff\"}"),
		"too many":      tooManyDocument,
		"trailing":      []byte(`{"server.name":"First"} {}`),
		"oversized":     []byte(`{"server.name":"` + string(bytes.Repeat([]byte("x"), maxStoredJSONSize)) + `"}`),
	}
}

func TestStoredJSONRequiresOwnerOnlyRegularFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), configurationFile)
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil { //nolint:gosec // Public permissions are the rejected boundary under test.
		t.Fatal(err)
	}
	if _, err := readJSON(path); err == nil {
		t.Fatal("public stored configuration was accepted")
	}
}
