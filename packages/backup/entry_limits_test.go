package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func archiveWithHeaders(t *testing.T, headers []*tar.Header, contents [][]byte) []byte {
	t.Helper()
	var output bytes.Buffer
	compressed := gzip.NewWriter(&output)
	archive := tar.NewWriter(compressed)
	for index, header := range headers {
		if err := archive.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if index < len(contents) {
			if _, err := archive.Write(contents[index]); err != nil {
				t.Fatal(err)
			}
		}
	}
	_ = archive.Flush()
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func TestRestoreRejectsOversizedEntryDocumentsWithoutSideEffects(t *testing.T) {
	for name, header := range map[string]*tar.Header{
		"entry document above its limit":  {Name: "progress.json", Mode: 0o600, Size: 256<<20 + 1},
		"whole document above its limit":  {Name: "settings.json", Mode: 0o600, Size: 16<<20 + 1},
		"unknown large member":            {Name: "progress.json.bak", Mode: 0o600, Size: 17 << 20},
		"entry limit on a whole document": {Name: "playlists.json", Mode: 0o600, Size: 17 << 20},
	} {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			original := []byte(`{"name":"Original"}`)
			if err := os.WriteFile(filepath.Join(directory, "settings.json"), original, 0o600); err != nil {
				t.Fatal(err)
			}
			archive := archiveWithHeaders(t, []*tar.Header{header}, nil)
			if err := testArchive.service.Restore(bytes.NewReader(archive), directory); err == nil {
				t.Fatal("oversized member accepted")
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 {
				t.Fatalf("restore left files: %v, %v", entries, err)
			}
			if data, err := os.ReadFile(filepath.Join(directory, "settings.json")); err != nil || !bytes.Equal(data, original) {
				t.Fatalf("restore changed settings: %q, %v", data, err)
			}
		})
	}
}

func TestRestoreBoundsWholeDocumentsSeparatelyFromEntryDocuments(t *testing.T) {
	directory := t.TempDir()
	large := []byte(`{"key":"` + strings.Repeat("v", 20<<20) + `"}`)
	if err := os.WriteFile(filepath.Join(directory, "kinosail.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	live := testArchive.service.WithLiveDatabase(exportedDocuments{"progress.json": large, "metadata.json": large, "settings.json": []byte(`{}`)})
	var archive bytes.Buffer
	if err := live.Write(&archive, directory, "dev"); err != nil {
		t.Fatalf("entry documents above the whole-document limit = %v", err)
	}
	restored := t.TempDir()
	if err := testArchive.service.Restore(bytes.NewReader(archive.Bytes()), restored); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(restored, "metadata.json")); err != nil || !bytes.Equal(data, large) {
		t.Fatalf("restored metadata = %d bytes, %v", len(data), err)
	}
}
