package backup_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

type exportedDocuments map[string][]byte

func (documents exportedDocuments) Export() (map[string][]byte, error) { return documents, nil }

func TestBackupRejectsDocumentsTheArchiveCannotRestore(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "kinosail.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	large := []byte(`"` + strings.Repeat("x", 16<<20) + `"`)
	live := testArchive.service.WithLiveDatabase(exportedDocuments{"collections.json": large, "settings.json": []byte(`{}`)})
	var archive bytes.Buffer
	if err := live.Write(&archive, directory, "dev"); err == nil || !strings.Contains(err.Error(), "collections.json") || archive.Len() != 0 {
		t.Fatalf("oversized document = %v with %d archive bytes", err, archive.Len())
	}
	live = testArchive.service.WithLiveDatabase(exportedDocuments{"unlisted.json": []byte(`{}`), "settings.json": []byte(`{}`)})
	if err := live.Write(&archive, directory, "dev"); err == nil || !strings.Contains(err.Error(), "unlisted.json") || archive.Len() != 0 {
		t.Fatalf("unlisted document = %v with %d archive bytes", err, archive.Len())
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

func TestBackupConfigurationRejectsDocumentsOutsideTheStateBoundary(t *testing.T) {
	for name, config := range map[string]sharedbackup.Config{
		"no documents":       {},
		"secret file":        {Documents: []string{"settings.json", "secrets.json"}},
		"manifest":           {Documents: []string{"manifest.json"}},
		"parent path":        {Documents: []string{"../settings.json"}},
		"nested path":        {Documents: []string{"state/settings.json"}},
		"not JSON":           {Documents: []string{"settings.yaml"}},
		"duplicate":          {Documents: []string{"settings.json", "settings.json"}},
		"unknown entry":      {Documents: []string{"settings.json"}, EntryDocuments: []string{"progress.json"}},
		"duplicate entry":    {Documents: []string{"progress.json", "settings.json"}, EntryDocuments: []string{"progress.json", "progress.json"}},
		"too many documents": {Documents: manyDocuments(257)},
	} {
		t.Run(name, func(t *testing.T) {
			config.DatabaseFilename, config.ValidateSettings = "kinosail.db", func([]byte) error { return nil }
			config.OpenDatabase = func(string) (sharedbackup.Database, error) { return nil, nil }
			if service, err := sharedbackup.New(config); err == nil || service != nil {
				t.Fatalf("configuration accepted: %v", err)
			}
		})
	}
}

func manyDocuments(count int) []string {
	names := make([]string, count)
	for index := range names {
		names[index] = "document-" + strconv.Itoa(index) + ".json"
	}
	return names
}
