package backup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestEncryptedBackupAuthenticatesAndRestoresExpandedState(t *testing.T) { //nolint:cyclop // One encrypted round trip checks state, secrets, and authentication.
	source, destination := t.TempDir(), t.TempDir()
	//nolint:gosec // G101: the fixture verifies that secret-shaped backup content is encrypted.
	for name, content := range map[string]string{"settings.json": `{"name":"Home"}`, "collections.json": `{"Favorites":{"name":"Favorites"}}`, "history.json": `[]`, "secrets.json": `{"backup.key":"hidden"}`, "viewing_imports.json": `{"sync":{"token":"private"}}`} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var archive bytes.Buffer
	if err := testArchive.WriteEncrypted(&archive, source, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(archive.Bytes(), []byte("Favorites")) {
		t.Fatal("encrypted archive contains plaintext")
	}
	if err := testArchive.RestoreEncrypted(bytes.NewReader(archive.Bytes()), destination, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(destination, "collections.json"))
	if err != nil || !bytes.Contains(data, []byte("Favorites")) {
		t.Fatalf("restored=%q err=%v", data, err)
	}
	if data, err = os.ReadFile(filepath.Join(destination, "viewing_imports.json")); err != nil || !bytes.Contains(data, []byte("private")) {
		t.Fatalf("restored viewing sync=%q err=%v", data, err)
	}
	if err := testArchive.RestoreEncrypted(bytes.NewReader(archive.Bytes()), t.TempDir(), "wrong passphrase value"); err == nil {
		t.Fatal("wrong passphrase succeeded")
	}
	if err := testArchive.VerifyEncrypted(bytes.NewReader(archive.Bytes()), "correct horse battery staple"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
