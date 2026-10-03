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
