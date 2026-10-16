package backup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type exportedDocuments map[string][]byte

func (documents exportedDocuments) Export() (map[string][]byte, error) { return documents, nil }

func TestBackupRejectsDocumentsTheArchiveCannotRestore(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "kinosail.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	large := []byte(`"` + strings.Repeat("x", 16<<20) + `"`)
	live := testArchive.service.WithLiveDatabase(exportedDocuments{"metadata.json": large, "settings.json": []byte(`{}`)})
	var archive bytes.Buffer
	if err := live.Write(&archive, directory, "dev"); err == nil || !strings.Contains(err.Error(), "metadata.json") || archive.Len() != 0 {
		t.Fatalf("oversized document = %v with %d archive bytes", err, archive.Len())
	}
	live = testArchive.service.WithLiveDatabase(exportedDocuments{"settings.json": []byte(`{"name":"Live"}`)})
	if err := live.Write(&archive, directory, "dev"); err != nil {
		t.Fatal(err)
	}
	restored := t.TempDir()
	if err := testArchive.service.Restore(bytes.NewReader(archive.Bytes()), restored); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(restored, "settings.json")); err != nil || string(data) != `{"name":"Live"}` {
		t.Fatalf("restored settings = %q, %v", data, err)
	}
}
