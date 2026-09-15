package appcli

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestConfigurationCommand(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
	path := ""
	validate := func(value string) (ConfigurationCounts, error) {
		path = value
		return ConfigurationCounts{Defaults: 1, UI: 2, File: 3, Environment: 4}, nil
	}
	handled, err := ConfigurationCommand([]string{"config", "validate", "settings.yaml"}, &output, validate)
	if !handled || err != nil || path != "settings.yaml" {
		t.Fatalf("validate = %v, %v, %q", handled, err, path)
	}
	if output.String() != "Configuration is valid: 1 defaults, 2 UI, 3 file, 4 environment.\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestConfigurationCommandRejectsInvalidInputBeforeValidation(t *testing.T) {
	t.Parallel()
	called := false
	validate := func(string) (ConfigurationCounts, error) { called = true; return ConfigurationCounts{}, nil }
	assertInvalidConfigurationArguments(t, validate)
	if called {
		t.Fatal("validator called for invalid input")
	}
	if handled, err := ConfigurationCommand(nil, io.Discard, validate); handled || err != nil {
		t.Fatalf("unrelated command = %v, %v", handled, err)
	}
	if handled, err := ConfigurationCommand([]string{"config", "validate"}, io.Discard, nil); !handled || err == nil {
		t.Fatalf("missing validator = %v, %v", handled, err)
	}
	want := errors.New("invalid configuration")
	if _, err := ConfigurationCommand([]string{"config", "validate"}, io.Discard, func(string) (ConfigurationCounts, error) { return ConfigurationCounts{}, want }); !errors.Is(err, want) {
		t.Fatalf("validation error = %v", err)
	}
	if _, err := ConfigurationCommand([]string{"config", "validate"}, errorWriter{}, validate); err == nil {
		t.Fatal("writer error was discarded")
	}
}

func assertInvalidConfigurationArguments(t *testing.T, validate func(string) (ConfigurationCounts, error)) {
	t.Helper()
	for _, args := range [][]string{{"config"}, {"config", "unknown"}, {"config", "validate", "one", "two"}} {
		if handled, err := ConfigurationCommand(args, io.Discard, validate); !handled || err == nil {
			t.Errorf("invalid arguments accepted: %#v", args)
		}
	}
}

func TestCountConfigurationSources(t *testing.T) {
	t.Parallel()
	fields := []struct{ source string }{{"default"}, {"ui"}, {"ui"}, {"file"}, {"environment"}, {"other"}}
	counts := CountConfigurationSources(fields, func(field struct{ source string }) string { return field.source }, "default", "ui", "file", "environment")
	if counts != (ConfigurationCounts{Defaults: 1, UI: 2, File: 1, Environment: 1}) {
		t.Fatalf("counts = %#v", counts)
	}
}

func TestBoundConfigurationCommandAdaptsTypedSnapshots(t *testing.T) {
	t.Parallel()
	type snapshot struct{ fields []string }
	command := BindConfigurationCommand(func(value snapshot) []string { return value.fields }, func(value string) string { return value }, "default", "ui", "file", "environment")
	var output bytes.Buffer
	handled, err := command([]string{"config", "validate"}, &output, func(string) (snapshot, error) {
		return snapshot{[]string{"default", "file"}}, nil
	})
	if !handled || err != nil || output.String() != "Configuration is valid: 1 defaults, 0 UI, 1 file, 0 environment.\n" {
		t.Fatalf("bound command = %v, %v, %q", handled, err, output.String())
	}
	want := errors.New("load failed")
	if _, err = command([]string{"config", "validate"}, io.Discard, func(string) (snapshot, error) { return snapshot{}, want }); !errors.Is(err, want) {
		t.Fatalf("bound load error = %v", err)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, errors.New(strings.Repeat("write failed", 1))
}
