package backup

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
)

func TestBackupRoundTripsProgressLargerThanAWholeDocument(t *testing.T) {
	directory := t.TempDir()
	store, err := database.Open(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.EnableEntries("progress.json"); err != nil {
		t.Fatal(err)
	}
	position := map[string]any{"seconds": 42, "session": strings.Repeat("s", 120), "updated": "2026-10-16T00:00:00Z"}
	want := make(map[string]map[string]any)
	batch := make(map[string]any)
	for index := 0; len(want)*260 <= database.DocumentSizeLimit*5/4; index++ {
		key := "0123456789abcdef0123456789abcdef:item-" + strconv.Itoa(index) + "-0123456789abcdef"
		want[key], batch[key] = position, position
		if len(batch) == 10_000 {
			if err := store.SaveEntries("progress.json", batch, nil); err != nil {
				t.Fatal(err)
			}
			batch = make(map[string]any)
		}
	}
	if err := store.SaveEntries("progress.json", batch, nil); err != nil {
		t.Fatal(err)
	}
	exported, err := store.Export()
	if err != nil || len(exported["progress.json"]) <= database.DocumentSizeLimit {
		t.Fatalf("progress fixture = %d bytes, %v", len(exported["progress.json"]), err)
	}
	const key = "large-entry-backup-key"
	var archive bytes.Buffer
	if err := LiveWriteEncrypted(store)(&archive, directory, key); err != nil {
		t.Fatalf("backup of large entry document = %v", err)
	}
	restored := t.TempDir()
	if err := service.RestoreAuto(bytes.NewReader(archive.Bytes()), restored, key); err != nil {
		t.Fatal(err)
	}
	reopened, err := database.Open(restored, false)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	var got map[string]map[string]any
	if found, err := reopened.LoadJSON("progress.json", &got); err != nil || !found || !reflect.DeepEqual(normalized(got), normalized(want)) {
		t.Fatalf("restored %d of %d records, found=%t, %v", len(got), len(want), found, err)
	}
	if err := reopened.SaveEntry("progress.json", "viewer:after-restore", position); err != nil {
		t.Fatalf("restored progress is not in entry storage: %v", err)
	}
}

func normalized(values map[string]map[string]any) map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		result[key] = value["session"].(string) + value["updated"].(string)
	}
	return result
}
