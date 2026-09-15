package commandtest

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/appcli"
)

func BackupRoundTrip(t *testing.T, command appcli.Command) {
	t.Parallel()

	source, restored := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.json"), []byte(`{"name":"Home","libraries":["."]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	handled, err := command([]string{"backup"}, nil, &archive, source, "", "", false, "")
	if !handled || err != nil {
		t.Fatalf("backup = %v, %v", handled, err)
	}
	handled, err = command([]string{"restore"}, bytes.NewReader(archive.Bytes()), io.Discard, restored, "", "", false, "")
	if !handled || err != nil {
		t.Fatalf("restore = %v, %v", handled, err)
	}
	data, err := os.ReadFile(filepath.Join(restored, "settings.json"))
	if err != nil || string(data) != `{"name":"Home","libraries":["."]}` {
		t.Fatalf("settings = %q, %v", data, err)
	}
}

func EncryptedRestore(t *testing.T, command appcli.Command, writeEncrypted func(io.Writer, string, string) error) {
	t.Parallel()
	source, restored := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "settings.json"), []byte(`{"name":"Encrypted Home"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := writeEncrypted(&archive, source, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	handled, err := command([]string{"restore"}, bytes.NewReader(archive.Bytes()), io.Discard, restored, "correct horse battery staple", "", false, "")
	data, readErr := os.ReadFile(filepath.Join(restored, "settings.json"))
	if !handled || err != nil || readErr != nil || string(data) != `{"name":"Encrypted Home"}` {
		t.Fatalf("restore=%v err=%v data=%q read=%v", handled, err, data, readErr)
	}
}

func RecoveryBackup(t *testing.T, command appcli.Command) { //nolint:cyclop // One recovery scenario covers backup, verification, and restore.
	t.Parallel()
	key := "correct horse battery staple"
	source, restored := t.TempDir(), t.TempDir()
	for name, value := range map[string]string{"settings.json": `{"name":"Home"}`, "secrets.json": `{"token":"private"}`} { //nolint:gosec // The credential-shaped value is inert test fixture data.
		if err := os.WriteFile(filepath.Join(source, name), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	if handled, err := command([]string{"backup"}, nil, &archive, source, key, "", false, ""); !handled || err != nil {
		t.Fatalf("backup = %v, %v", handled, err)
	}
	if handled, err := command([]string{"backup", "verify"}, bytes.NewReader(archive.Bytes()), io.Discard, source, key, "", false, ""); !handled || err != nil {
		t.Fatalf("verify = %v, %v", handled, err)
	}
	if handled, err := command([]string{"backup", "verify"}, strings.NewReader("damaged"), io.Discard, source, key, "", false, ""); !handled || err == nil {
		t.Fatalf("damaged verify = %v, %v", handled, err)
	}
	if handled, err := command([]string{"restore"}, bytes.NewReader(archive.Bytes()), io.Discard, restored, key, "", false, ""); !handled || err != nil {
		t.Fatalf("restore = %v, %v", handled, err)
	}
	if private, err := os.ReadFile(filepath.Join(restored, "secrets.json")); err != nil || string(private) != `{"token":"private"}` {
		t.Fatalf("private state = %q, %v", private, err)
	}
}

func Version(t *testing.T, command appcli.Command) {
	t.Parallel()

	var output bytes.Buffer
	handled, err := command([]string{"version"}, nil, &output, t.TempDir(), "", "", false, "")
	if !handled || err != nil || output.String() != "dev\n" {
		t.Fatalf("version = %v, %v, %q", handled, err, output.String())
	}
}
