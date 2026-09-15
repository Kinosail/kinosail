package backup

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBoundCommandOwnsRecoveryDispatch(t *testing.T) { //nolint:cyclop // One command contract remains below the repository complexity ceiling.
	service := testService(t, func(string) (Database, error) { return nil, nil }, func([]byte) error { return nil })
	source, restored := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.json"), []byte(`{"name":"Home"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	versionCalls := 0
	command := BindCommand(service, func() string { versionCalls++; return "dev" })
	var archive bytes.Buffer
	if handled, err := command([]string{"backup"}, nil, &archive, source, ""); !handled || err != nil {
		t.Fatalf("backup = %v, %v", handled, err)
	}
	if handled, err := command([]string{"backup", "verify"}, bytes.NewReader(archive.Bytes()), io.Discard, source, ""); !handled || err != nil {
		t.Fatalf("verify = %v, %v", handled, err)
	}
	if handled, err := command([]string{"restore"}, bytes.NewReader(archive.Bytes()), io.Discard, restored, ""); !handled || err != nil {
		t.Fatalf("restore = %v, %v", handled, err)
	}
	if data, err := os.ReadFile(filepath.Join(restored, "settings.json")); err != nil || string(data) != `{"name":"Home"}` {
		t.Fatalf("restored settings = %q, %v", data, err)
	}
	if versionCalls != 1 {
		t.Fatalf("version calls = %d", versionCalls)
	}
	if handled, err := command([]string{"version"}, nil, io.Discard, source, ""); handled || err != nil {
		t.Fatalf("unrelated command = %v, %v", handled, err)
	}
	if handled, err := command(nil, nil, io.Discard, source, ""); handled || err != nil {
		t.Fatalf("empty command = %v, %v", handled, err)
	}
}

func TestBoundCommandRejectsInvalidCommandsBeforeEffects(t *testing.T) {
	service := testService(t, func(string) (Database, error) { return nil, nil }, func([]byte) error { return nil })
	command := BindCommand(service, func() string { return "dev" })
	for name, test := range map[string]struct {
		args   []string
		input  io.Reader
		output io.Writer
	}{
		"missing backup output": {args: []string{"backup"}},
		"missing verify input":  {args: []string{"backup", "verify"}, output: io.Discard},
		"missing restore input": {args: []string{"restore"}, output: io.Discard},
		"backup arguments":      {args: []string{"backup", "extra", "value"}, input: strings.NewReader("unused"), output: io.Discard},
		"restore arguments":     {args: []string{"restore", "extra"}, input: strings.NewReader("unused"), output: io.Discard},
	} {
		t.Run(name, func(t *testing.T) {
			if handled, err := command(test.args, test.input, test.output, t.TempDir(), ""); !handled || err == nil {
				t.Fatalf("invalid command = %v, %v", handled, err)
			}
		})
	}
	for name, invalid := range map[string]Command{
		"service": BindCommand(nil, func() string { return "dev" }),
		"version": BindCommand(service, nil),
	} {
		if handled, err := invalid([]string{"backup"}, nil, io.Discard, t.TempDir(), ""); !handled || err == nil {
			t.Fatalf("missing %s = %v, %v", name, handled, err)
		}
	}
}
