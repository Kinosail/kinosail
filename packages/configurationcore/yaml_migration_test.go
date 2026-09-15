package configurationcore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLRejectedWithoutReadingSecrets(t *testing.T) {
	for name, content := range map[string]string{
		"empty": "", "malformed": "version: [", "missing version": "name: household",
		"unknown": "version: 1\nunknown: value", "duplicate": "version: 1\nname: a\nname: b",
		"wrong scalar": "version: 1\ncount: [1]", "conflicting": "version: 1\npaths.data: one\npaths: {data: two}",
		"extra document": "version: 1\n---\nversion: 1", "empty extra document": "version: 1\n---",
		"recursive alias": "version: 1\nname: &x [*x]", "oversized": strings.Repeat("x", maxYAMLSize+1),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			loader := testLoader()
			loader.ReadStored = func(string) (map[string]string, map[string]string, error) {
				return nil, nil, nil
			}
			loader.ReadSecret = func(string) ([]byte, error) { t.Fatal("read secret for rejected input"); return nil, nil }
			if values, err := loader.Load("data", path, func(string) (string, bool) { return "", false }); err == nil || values != nil {
				t.Fatalf("accepted %q: %v", name, values)
			}
		})
	}
}

func TestYAMLAliasesAndDocumentEndRemainSupported(t *testing.T) {
	values, err := testLoader().decodeYAML(strings.NewReader("version: 1\nname: &name household\npaths: {data: *name}\n...\n"))
	if err != nil || values["name"] != "household" || values["paths.data"] != "household" {
		t.Fatalf("%v %v", values, err)
	}
}
