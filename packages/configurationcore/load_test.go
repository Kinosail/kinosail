package configurationcore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testLoader() Loader {
	return Loader{
		Schema: Schema{
			Fields: []Field{
				{Key: "paths.data", Env: "KINOSAIL_DATA_DIR", Default: "/default", Kind: Text},
				{Key: "name", Env: "NAME", Default: "base", Kind: Text},
				{Key: "enabled", Env: "ENABLED", Default: "false", Kind: Boolean},
				{Key: "count", Env: "COUNT", Default: "1", Kind: Number},
				{Key: "hosts", Env: "HOSTS", Default: "[]", Kind: List},
				{Key: "token", Env: "TOKEN", Kind: Text, Secret: true},
			},
			Retired: func(key string) bool { return strings.HasPrefix(key, "retired") },
			Validate: func(key, raw string) error {
				if raw == "invalid" {
					return errors.New("invalid " + key)
				}
				return nil
			},
		},
		ReadStored: func(string) (map[string]string, map[string]string, error) {
			return map[string]string{}, map[string]string{}, nil
		},
		ReadSecret: os.ReadFile,
	}
}

func TestLoaderResolvesPrecedenceAndTypes(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	secretPath := filepath.Join(directory, "token")
	if err := os.WriteFile(secretPath, []byte("yaml-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	yamlPath := filepath.Join(directory, "configuration.yaml")
	yaml := "version: 1\npaths:\n  data: yaml-data\nname: yaml\nenabled: true\ncount: 4\nhosts: [one, two]\ntoken_file: token\nretired.setting: old\n"
	if err := os.WriteFile(yamlPath, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	loader := testLoader()
	loader.ReadStored = func(dataDir string) (map[string]string, map[string]string, error) {
		if dataDir != "environment-data" {
			t.Fatalf("stored data directory = %q", dataDir)
		}
		return map[string]string{"name": "stored"}, map[string]string{"token": "stored-token"}, nil
	}
	environment := map[string]string{"KINOSAIL_DATA_DIR": "environment-data", "NAME": "environment"}
	values, err := loader.Load("argument-data", yamlPath, func(key string) (string, bool) {
		value, ok := environment[key]
		return value, ok
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Value{
		"paths.data": {Raw: "environment-data", Source: Environment},
		"name":       {Raw: "environment", Source: Environment},
		"enabled":    {Raw: "true", Source: YAML},
		"count":      {Raw: "4", Source: YAML},
		"hosts":      {Raw: `["one","two"]`, Source: YAML},
		"token":      {Raw: "yaml-token", Source: YAML},
	}
	for key, expected := range want {
		if values[key] != expected {
			t.Errorf("%s = %#v, want %#v", key, values[key], expected)
		}
	}
}

func TestLoaderRejectsLayerErrors(t *testing.T) {
	t.Parallel()
	base := testLoader()
	lookup := func(string) (string, bool) { return "", false }
	readError := errors.New("read failed")
	validationError := errors.New("invalid name")
	tests := map[string]Loader{
		"stored read": func() Loader {
			loader := base
			loader.ReadStored = func(string) (map[string]string, map[string]string, error) { return nil, nil, readError }
			return loader
		}(),
		"stored value": func() Loader {
			loader := base
			loader.ReadStored = func(string) (map[string]string, map[string]string, error) {
				return map[string]string{"unknown": "x"}, nil, nil
			}
			return loader
		}(),
		"environment": func() Loader {
			loader := base
			loader.Schema.Validate = func(string, string) error { return validationError }
			return loader
		}(),
	}
	for name, loader := range tests {
		t.Run(name, func(t *testing.T) {
			activeLookup := lookup
			if name == "environment" {
				activeLookup = func(key string) (string, bool) { return "value", key == "NAME" }
			}
			if _, err := loader.Load("data", "", activeLookup); err == nil {
				t.Fatal("invalid layer was accepted")
			}
		})
	}
	directory := t.TempDir()
	invalidYAML := filepath.Join(directory, "invalid.yaml")
	if err := os.WriteFile(invalidYAML, []byte("version: 1\nunknown: value\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := base.Load("data", invalidYAML, lookup); err == nil {
		t.Fatal("unknown YAML was accepted")
	}
	if _, err := base.Load("data", filepath.Join(directory, "missing.yaml"), lookup); err == nil {
		t.Fatal("missing YAML was accepted")
	}
}

func TestStoredAndYAMLValidation(t *testing.T) {
	t.Parallel()
	loader := testLoader()
	for name, stored := range map[string]struct {
		regular, secrets map[string]string
	}{
		"unknown":          {map[string]string{"unknown": "x"}, nil},
		"public in secret": {nil, map[string]string{"name": "x"}},
		"secret in public": {map[string]string{"token": "x"}, nil},
		"invalid":          {map[string]string{"name": "invalid"}, nil},
	} {
		t.Run(name, func(t *testing.T) {
			if err := loader.applyStored(loader.defaults(), stored.regular, stored.secrets); err == nil {
				t.Fatal("invalid stored value was accepted")
			}
		})
	}
	values := loader.defaults()
	if err := loader.applyStored(values, map[string]string{"retired.value": "x", "name": "saved"}, map[string]string{"token": "saved-token"}); err != nil {
		t.Fatal(err)
	}
	if values["name"].Source != GUI || values["token"].Source != GUI {
		t.Fatalf("stored sources = %#v", values)
	}

	if err := loader.applyYAML(values, map[string]string{"retired.value": "x"}, "configuration.yaml"); err != nil {
		t.Fatal(err)
	}
	for name, yamlValues := range map[string]map[string]string{
		"unknown":             {"unknown": "x"},
		"unknown secret file": {"unknown_file": "token"},
		"public secret file":  {"name_file": "token"},
		"invalid":             {"name": "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := loader.applyYAML(values, yamlValues, "configuration.yaml"); err == nil {
				t.Fatal("invalid YAML value was accepted")
			}
		})
	}
	loader.ReadSecret = func(string) ([]byte, error) { return nil, errors.New("secret failed") }
	if err := loader.applyYAML(values, map[string]string{"token_file": "/absolute/token"}, "configuration.yaml"); err == nil {
		t.Fatal("unreadable secret was accepted")
	}
}

func TestEnvironmentValidation(t *testing.T) {
	t.Parallel()
	loader := testLoader()
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) { value, ok := values[key]; return value, ok }
	}
	for name, environment := range map[string]map[string]string{
		"direct and file": {"NAME": "direct", "NAME_FILE": "file"},
		"public file":     {"NAME_FILE": "file"},
		"invalid direct":  {"NAME": "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := loader.applyEnvironment(loader.defaults(), lookup(environment)); err == nil {
				t.Fatal("invalid environment was accepted")
			}
		})
	}
	loader.ReadSecret = func(string) ([]byte, error) { return nil, errors.New("secret failed") }
	if err := loader.applyEnvironment(loader.defaults(), lookup(map[string]string{"TOKEN_FILE": "missing"})); err == nil {
		t.Fatal("unreadable environment secret was accepted")
	}
	loader.ReadSecret = func(string) ([]byte, error) { return []byte("secret\n"), nil }
	values := loader.defaults()
	if err := loader.applyEnvironment(values, lookup(map[string]string{"TOKEN_FILE": "token"})); err != nil || values["token"] != (Value{Raw: "secret", Source: Environment}) {
		t.Fatalf("secret environment = %#v, %v", values["token"], err)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func TestYAMLBoundaryAndShapes(t *testing.T) {
	t.Parallel()
	loader := testLoader()
	if values, err := loader.readYAML(""); err != nil || len(values) != 0 {
		t.Fatalf("empty YAML = %#v, %v", values, err)
	}
	if _, err := loader.readYAML(t.TempDir()); err == nil {
		t.Fatal("directory was accepted as YAML")
	}
	large := filepath.Join(t.TempDir(), "large.yaml")
	if err := os.WriteFile(large, make([]byte, maxYAMLSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loader.readYAML(large); err == nil {
		t.Fatal("oversized YAML file was accepted")
	}
	for name, reader := range map[string]any{
		"read failure": failingReader{},
		"oversized":    strings.NewReader(strings.Repeat("x", maxYAMLSize+1)),
		"malformed":    strings.NewReader("["),
		"version":      strings.NewReader("version: 2"),
		"nested error": strings.NewReader("version: 1\nparent:\n  child: true"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loader.decodeYAML(reader.(interface{ Read([]byte) (int, error) })); err == nil {
				t.Fatal("invalid YAML was accepted")
			}
		})
	}
	if _, err := loader.decodeYAML(strings.NewReader("version: 1\nparent:\n  child: value\nparent.child: other\n")); err == nil {
		t.Fatal("ambiguous YAML was accepted")
	}
}

func TestYAMLValueShapes(t *testing.T) {
	t.Parallel()
	loader := testLoader()
	tests := []struct {
		name  string
		value any
		want  string
		ok    bool
	}{
		{"name", "text", "text", true},
		{"enabled", true, "true", true},
		{"count", 2, "2", true},
		{"hosts", []any{"a", "b"}, `["a","b"]`, true},
		{"retired.value", "old", "old", true},
		{"retired.value", true, "", false},
		{"unknown", true, "", false},
		{"name", true, "", false},
		{"name", 2, "", false},
		{"name", []any{"a"}, "", false},
		{"hosts", []any{"a", 2}, "", false},
		{"name", nil, "", false},
		{"token_file", "file", "file", true},
		{"unknown_file", "file", "", false},
		{"name_file", "file", "", false},
	}
	for _, test := range tests {
		raw, err := loader.yamlValue(test.name, test.value)
		if (err == nil) != test.ok || raw != test.want {
			t.Errorf("yamlValue(%q, %#v) = %q, %v", test.name, test.value, raw, err)
		}
	}
	output := map[string]string{"name": "first"}
	if err := loader.flatten("", map[string]any{"name": "second"}, output); err == nil {
		t.Fatal("duplicate flattened setting was accepted")
	}
	if _, ok := loader.find("missing"); ok || !loader.retired("retired.value") {
		t.Fatal("schema lookup returned the wrong result")
	}
	loader.Schema.Retired = nil
	if loader.retired("retired.value") {
		t.Fatal("nil retirement policy retired a setting")
	}
}
