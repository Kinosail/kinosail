package archivetest

import (
	"archive/tar"
	"archive/zip"
	"os"
	"testing"
)

// WriteZIP creates a ZIP fixture without filtering intentionally invalid test entries.
func WriteZIP(t *testing.T, name string, files map[string]string) {
	t.Helper()
	file, err := os.Create(name) //nolint:gosec // Test archive path is inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	archive := zip.NewWriter(file)
	for name, content := range files {
		entry, createErr := archive.Create(name)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entry.Write([]byte(content)); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

// WriteTar creates a TAR fixture with the original Player entry modes and content.
func WriteTar(t *testing.T, name string, files map[string]string) {
	t.Helper()
	file, err := os.Create(name) //nolint:gosec // Test archive path is inside t.TempDir.
	if err != nil {
		t.Fatal(err)
	}
	archive := tar.NewWriter(file)
	for name, content := range files {
		if err := archive.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := archive.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
