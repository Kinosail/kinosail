package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupRejectsMissingOrInvalidConfigurationState(t *testing.T) {
	for name, prepare := range map[string]func(*testing.T) string{
		"missing": func(t *testing.T) string { return t.TempDir() },
		"invalid JSON": func(t *testing.T) string {
			directory := t.TempDir()
			writeBackupFile(t, filepath.Join(directory, "settings.json"), "{")
			return directory
		},
		"oversized": func(t *testing.T) string {
			directory := t.TempDir()
			writeBackupFile(t, filepath.Join(directory, "settings.json"), `"`+strings.Repeat("x", 16<<20)+`"`)
			return directory
		},
		"unreadable path": func(t *testing.T) string {
			path := filepath.Join(t.TempDir(), "not-a-directory")
			writeBackupFile(t, path, "file")
			return path
		},
		"symlink": func(t *testing.T) string {
			directory := t.TempDir()
			outside := filepath.Join(t.TempDir(), "outside.json")
			writeBackupFile(t, outside, `{"name":"Outside"}`)
			if err := os.Symlink(outside, filepath.Join(directory, "settings.json")); err != nil {
				t.Fatal(err)
			}
			return directory
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := testArchive.Write(io.Discard, prepare(t)); err == nil {
				t.Fatal("invalid backup source was accepted")
			}
		})
	}

	directory := t.TempDir()
	writeBackupFile(t, filepath.Join(directory, "settings.json"), `{}`)
	if err := testArchive.Write(errorWriter{}, directory); err == nil {
		t.Fatal("archive writer failure was ignored")
	}
	if err := testArchive.WriteEncrypted(io.Discard, directory, "short"); err == nil {
		t.Fatal("short backup passphrase was accepted")
	}
	for call := 1; call <= 4; call++ {
		if err := testArchive.WriteEncrypted(&failCallWriter{fail: call}, directory, "correct horse battery staple"); err == nil {
			t.Fatalf("encrypted backup ignored writer failure %d", call)
		}
	}
}

func TestWriteAutoSelectsEncryptedOrPlainArchive(t *testing.T) {
	directory := t.TempDir()
	writeBackupFile(t, filepath.Join(directory, "settings.json"), `{}`)
	for name, passphrase := range map[string]string{
		"plain":     "",
		"encrypted": "correct horse battery staple",
	} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			if err := testArchive.WriteAuto(&archive, directory, passphrase); err != nil {
				t.Fatal(err)
			}
			if archive.Len() == 0 {
				t.Fatal("automatic backup produced no archive")
			}
		})
	}
}

func TestRestoreRejectsMalformedArchiveContracts(t *testing.T) { //nolint:cyclop // Each fixture is one externally observable archive contract violation.
	validManifest := `{"format":1,"kinosailVersion":"dev","stateSchema":1,"createdAt":"2026-08-23T00:00:00Z","files":["settings.json"],"mediaIncluded":false}`
	for name, archive := range map[string][]byte{
		"not gzip":              []byte("not gzip"),
		"empty":                 tarArchive(t, nil),
		"directory":             tarArchive(t, []archiveEntry{{name: "settings.json", kind: tar.TypeDir, data: `{}`}}),
		"unknown":               tarArchive(t, []archiveEntry{{name: "unknown.json", data: `{}`}}),
		"secret in plain":       tarArchive(t, []archiveEntry{{name: "secrets.json", data: `{}`}}),
		"duplicate":             tarArchive(t, []archiveEntry{{name: "settings.json", data: `{}`}, {name: "settings.json", data: `{}`}}),
		"invalid JSON":          tarArchive(t, []archiveEntry{{name: "settings.json", data: `{`}}),
		"invalid manifest":      tarArchive(t, []archiveEntry{{name: "settings.json", data: `{}`}, {name: "manifest.json", data: `{}`}}),
		"invalid manifest date": tarArchive(t, []archiveEntry{{name: "settings.json", data: `{}`}, {name: "manifest.json", data: strings.Replace(validManifest, "2026-08-23T00:00:00Z", "yesterday", 1)}}),
		"manifest mismatch":     tarArchive(t, []archiveEntry{{name: "settings.json", data: `{}`}, {name: "manifest.json", data: strings.Replace(validManifest, `["settings.json"]`, `[]`, 1)}}),
		"manifest includes media": tarArchive(t, []archiveEntry{{name: "settings.json", data: `{}`}, {
			name: "manifest.json", data: strings.Replace(validManifest, `"mediaIncluded":false`, `"mediaIncluded":true`, 1),
		}}),
	} {
		t.Run(name, func(t *testing.T) {
			if err := testArchive.Restore(bytes.NewReader(archive), t.TempDir()); err == nil {
				t.Fatal("invalid archive was restored")
			}
		})
	}

	oversized := tarArchive(t, []archiveEntry{{name: "settings.json", data: `"` + strings.Repeat("x", 16<<20) + `"`}})
	if err := testArchive.Restore(bytes.NewReader(oversized), t.TempDir()); err == nil {
		t.Fatal("oversized entry was restored")
	}
}

func TestRestoreSupportsLegacyAndAutoDetectedArchives(t *testing.T) {
	legacy := tarArchive(t, []archiveEntry{{name: "settings.json", data: `{"name":"Legacy"}`}})
	destination := t.TempDir()
	if err := testArchive.RestoreAuto(bytes.NewReader(legacy), destination, ""); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(destination, "settings.json")); err != nil || !bytes.Contains(data, []byte("Legacy")) {
		t.Fatalf("legacy restore = %q, %v", data, err)
	}

	source := t.TempDir()
	writeBackupFile(t, filepath.Join(source, "settings.json"), `{"name":"Encrypted"}`)
	var encrypted bytes.Buffer
	if err := testArchive.WriteEncrypted(&encrypted, source, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.RestoreAuto(bytes.NewReader(encrypted.Bytes()), t.TempDir(), "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range [][]byte{nil, []byte("KINOSAIL-BACKUP-1\nshort")} {
		if err := testArchive.VerifyEncrypted(bytes.NewReader(invalid), "correct horse battery staple"); err == nil {
			t.Fatal("invalid encrypted backup was verified")
		}
	}
}

func TestVerifyAutoAcceptsPlainAndEncryptedArchives(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	writeBackupFile(t, filepath.Join(source, "settings.json"), `{"name":"Home"}`)
	var plain, encrypted bytes.Buffer
	if err := testArchive.Write(&plain, source); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.WriteEncrypted(&encrypted, source, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.VerifyAuto(bytes.NewReader(plain.Bytes()), ""); err != nil {
		t.Fatalf("plain verification: %v", err)
	}
	if err := testArchive.VerifyAuto(bytes.NewReader(encrypted.Bytes()), "correct horse battery staple"); err != nil {
		t.Fatalf("encrypted verification: %v", err)
	}
	if err := testArchive.VerifyAuto(bytes.NewReader(encrypted.Bytes()), "wrong passphrase value"); err == nil {
		t.Fatal("encrypted backup verified with the wrong passphrase")
	}
	corrupt := append([]byte(nil), plain.Bytes()...)
	corrupt[len(corrupt)-1] ^= 0xff
	if err := testArchive.VerifyAuto(bytes.NewReader(corrupt), ""); err == nil {
		t.Fatal("archive with an invalid gzip checksum was verified")
	}
}

func TestBackupValidatesAuditJournalAndEncryptedSecrets(t *testing.T) {
	directory := t.TempDir()
	writeBackupFile(t, filepath.Join(directory, "settings.json"), `{}`)
	writeBackupFile(t, filepath.Join(directory, "audit.jsonl"), "{\"action\":\"one\"}\n{\"action\":\"two\"}\n")
	writeBackupFile(t, filepath.Join(directory, "secrets.json"), `{"token":"private"}`)
	var plain bytes.Buffer
	if err := testArchive.Write(&plain, directory); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(plain.Bytes(), []byte("private")) {
		t.Fatal("plain backup contains secret state")
	}
	var encrypted bytes.Buffer
	if err := testArchive.WriteEncrypted(&encrypted, directory, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	restored := t.TempDir()
	if err := testArchive.RestoreEncrypted(bytes.NewReader(encrypted.Bytes()), restored, "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	if secret, err := os.ReadFile(filepath.Join(restored, "secrets.json")); err != nil || string(secret) != `{"token":"private"}` {
		t.Fatalf("secret restore = %q, %v", secret, err)
	}

	for name, contents := range map[string]string{"empty": "", "invalid line": "{}\n{\n"} {
		t.Run(name, func(t *testing.T) {
			source := t.TempDir()
			writeBackupFile(t, filepath.Join(source, "audit.jsonl"), contents)
			if err := testArchive.Write(io.Discard, source); err == nil {
				t.Fatal("invalid audit journal was archived")
			}
		})
	}
}

func TestAutomaticBackupDetectionBoundsAndPropagatesReaderFailures(t *testing.T) {
	for _, verify := range []func(io.Reader) error{
		func(reader io.Reader) error { return testArchive.VerifyAuto(reader, "correct horse battery staple") },
		func(reader io.Reader) error {
			return testArchive.RestoreAuto(reader, t.TempDir(), "correct horse battery staple")
		},
	} {
		if err := verify(failingReader{}); err == nil {
			t.Fatal("reader failure was ignored")
		}
		if err := verify(bytes.NewReader(make([]byte, (40<<20)+1))); err == nil {
			t.Fatal("oversized archive was accepted")
		}
	}
}

type archiveEntry struct {
	name string
	kind byte
	data string
}

func tarArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var result bytes.Buffer
	gzipWriter := gzip.NewWriter(&result)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, entry := range entries {
		kind := entry.kind
		if kind == 0 {
			kind = tar.TypeReg
		}
		size := int64(len(entry.data))
		if kind != tar.TypeReg {
			size = 0
		}
		if err := tarWriter.WriteHeader(&tar.Header{Name: entry.name, Typeflag: kind, Mode: 0o600, Size: size}); err != nil {
			t.Fatal(err)
		}
		if kind == tar.TypeReg {
			if _, err := tarWriter.Write([]byte(entry.data)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return result.Bytes()
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type failCallWriter struct {
	calls int
	fail  int
}

func (writer *failCallWriter) Write(data []byte) (int, error) {
	writer.calls++
	if writer.calls == writer.fail {
		return 0, errors.New("write failed")
	}
	return len(data), nil
}

func writeBackupFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
