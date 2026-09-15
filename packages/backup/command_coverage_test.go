package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/scrypt"
)

func TestBackupCoverageClosesArchiveFailureEdges(t *testing.T) { //nolint:cyclop // One matrix covers archive input and output failures.
	service := testService(t, func(string) (Database, error) { return nil, nil }, func([]byte) error { return nil })
	directory := t.TempDir()
	if err := os.Mkdir(filepath.Join(directory, service.databaseFilename), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := service.Write(io.Discard, directory, "dev"); err == nil {
		t.Fatal("directory database was accepted")
	}
	notDirectory := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(notDirectory, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := service.Write(io.Discard, filepath.Join(notDirectory, "child"), "dev"); err == nil {
		t.Fatal("invalid data path was accepted")
	}

	unreadable := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(unreadable, []byte(`{}`), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := readStateFile(unreadable, maxFileSize); err == nil {
		t.Skip("filesystem permits reads from mode 000 files")
	}

	if err := service.Restore(bytes.NewReader(gzipBytes(t, []byte("short"))), t.TempDir()); err == nil {
		t.Fatal("truncated tar archive was accepted")
	}
	corruptTail := append(gzipBytes(t, make([]byte, 1024)), []byte("invalid gzip stream")...)
	if err := service.Restore(bytes.NewReader(corruptTail), t.TempDir()); err == nil {
		t.Fatal("corrupt trailing stream was accepted")
	}
	if err := service.Restore(bytes.NewReader(gzipBytes(t, make([]byte, 1024))), t.TempDir()); err == nil {
		t.Fatal("empty archive was accepted")
	}
	var oversized bytes.Buffer
	first := gzip.NewWriter(&oversized)
	if _, err := first.Write(make([]byte, 1024)); err != nil || first.Close() != nil {
		t.Fatal(err)
	}
	second := gzip.NewWriter(&oversized)
	if _, err := io.CopyN(second, zeroReader{}, maxDecodedSize+1); err != nil || second.Close() != nil {
		t.Fatal(err)
	}
	if err := service.Restore(bytes.NewReader(oversized.Bytes()), t.TempDir()); err == nil {
		t.Fatal("oversized decoded archive was accepted")
	}

	assertArchiveWriterFailures(t, service)
}

func assertArchiveWriterFailures(t *testing.T, service *Service) {
	t.Helper()
	state := t.TempDir()
	large := make([]byte, 1<<20)
	_, _ = rand.New(rand.NewSource(7)).Read(large) //nolint:gosec // Deterministic noise forces streaming writes in this failure test.
	content := `"` + base64.StdEncoding.EncodeToString(large) + `"`
	if err := os.WriteFile(filepath.Join(state, "configuration.json"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	counter := &countingWriter{}
	if err := service.Write(counter, state, "dev"); err != nil {
		t.Fatal(err)
	}
	for call := 1; call <= counter.calls; call++ {
		if err := service.Write(&writeFailure{failAt: call}, state, "dev"); err == nil {
			t.Fatalf("writer failure %d was discarded", call)
		}
	}
	for call := 1; call <= 2; call++ {
		writer := tar.NewWriter(&writeFailure{failAt: call})
		if err := writeArchiveManifest(writer, "dev", nil); err == nil {
			t.Fatalf("manifest writer failure %d was discarded", call)
		}
	}
}

func TestBackupCoverageClosesEncryptedFailureEdges(t *testing.T) { //nolint:cyclop // One matrix covers entropy, output, decrypt, and restore failures.
	service := testService(t, func(string) (Database, error) { return nil, nil }, func([]byte) error { return nil })
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	key := "correct horse battery staple"
	service.random = readFailure{}
	if err := service.WriteEncrypted(io.Discard, directory, key, "dev"); err == nil {
		t.Fatal("salt entropy failure was discarded")
	}
	service.random = io.MultiReader(bytes.NewReader(make([]byte, 16)), readFailure{})
	if err := service.WriteEncrypted(io.Discard, directory, key, "dev"); err == nil {
		t.Fatal("nonce entropy failure was discarded")
	}
	service.random = bytes.NewReader(make([]byte, 28))
	for call := 1; call <= 4; call++ {
		service.random = bytes.NewReader(make([]byte, 28))
		if err := service.WriteEncrypted(&writeFailure{failAt: call}, directory, key, "dev"); err == nil {
			t.Fatalf("encrypted writer failure %d was discarded", call)
		}
	}
	if err := service.VerifyEncrypted(readFailure{}, key); err == nil {
		t.Fatal("encrypted read failure was accepted")
	}
	if err := service.VerifyEncrypted(strings.NewReader("invalid"), "short"); err == nil {
		t.Fatal("short decrypt passphrase was accepted")
	}
	invalidPlain := encryptBackupPayload(t, []byte("not a gzip archive"), key)
	if err := service.RestoreEncrypted(bytes.NewReader(invalidPlain), t.TempDir(), key); err == nil {
		t.Fatal("invalid encrypted payload was restored")
	}
	valid := encryptBackupPayload(t, gzipBytes(t, make([]byte, 1024)), key)
	want := errors.New("use failed")
	if err := service.decrypt(bytes.NewReader(valid), key, func([]byte) error { return want }); !errors.Is(err, want) {
		t.Fatalf("decrypt callback error = %v", err)
	}
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var output bytes.Buffer
	writer := gzip.NewWriter(&output)
	if _, err := writer.Write(data); err != nil || writer.Close() != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func encryptBackupPayload(t *testing.T, plain []byte, passphrase string) []byte {
	t.Helper()
	salt, nonce := make([]byte, 16), make([]byte, 12)
	key, err := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	data := append([]byte(nil), encryptedMagic...)
	data = append(data, salt...)
	data = append(data, nonce...)
	return append(data, aead.Seal(nil, nonce, plain, encryptedMagic)...)
}

type countingWriter struct{ calls int }

func (writer *countingWriter) Write(data []byte) (int, error) {
	writer.calls++
	return len(data), nil
}

type writeFailure struct {
	calls, failAt int
}

func (writer *writeFailure) Write(data []byte) (int, error) {
	writer.calls++
	if writer.calls == writer.failAt {
		return 0, errors.New("write failed")
	}
	return len(data), nil
}

type readFailure struct{}

func (readFailure) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type zeroReader struct{}

func (zeroReader) Read(data []byte) (int, error) {
	clear(data)
	return len(data), nil
}
