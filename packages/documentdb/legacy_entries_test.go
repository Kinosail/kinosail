package documentdb

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

var entryImportConfig = Config{Documents: []string{"progress.json", "settings.json"}, EntryDocuments: []string{"progress.json"}}

func largeEntryDocument(t *testing.T, tail string) ([]byte, map[string]string) {
	t.Helper()
	value := strings.Repeat("p", 900)
	want := make(map[string]string)
	var document bytes.Buffer
	document.WriteString("{\n")
	for index := 0; document.Len() <= DocumentSizeLimit; index++ {
		key := "viewer:item-" + strconv.Itoa(index)
		want[key] = value
		if index > 0 {
			document.WriteString(",\n")
		}
		document.WriteString(`"` + key + `": "` + value + `"`)
	}
	document.WriteString(tail + "\n}")
	return document.Bytes(), want
}

func TestLargeLegacyEntryDocumentImportsIntoEntryStorage(t *testing.T) {
	directory := t.TempDir()
	document, want := largeEntryDocument(t, "")
	legacy := filepath.Join(directory, "progress.json")
	if err := os.WriteFile(legacy, document, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(directory, false, entryImportConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := os.Lstat(legacy); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("imported legacy file remains: %v", err)
	}
	if err := store.SaveEntry("progress.json", "viewer:new", "added"); err != nil {
		t.Fatalf("imported document is not in entry storage: %v", err)
	}
	want["viewer:new"] = "added"
	var got map[string]string
	if found, err := store.LoadJSON("progress.json", &got); err != nil || !found || !reflect.DeepEqual(got, want) {
		t.Fatalf("imported %d of %d records, found=%t, %v", len(got), len(want), found, err)
	}
}

func TestInvalidLargeLegacyEntryDocumentsAreRejectedWithoutSideEffects(t *testing.T) {
	cases := map[string]string{
		"duplicate key":      `,"viewer:item-0":"again"`,
		"escaped duplicate":  `,"viewer:item-0":"again"`,
		"nested duplicate":   `,"viewer:nested":{"a":1,"a":2}`,
		"invalid utf8 key":   ",\"viewer:\xff\":1",
		"invalid utf8 value": ",\"viewer:bad\":\"\xc3\"",
		"oversized entry":    `,"viewer:large":"` + strings.Repeat("x", EntrySizeLimit) + `"`,
		"empty key":          `,"":1`,
		"trailing data":      "",
	}
	for name, tail := range cases {
		t.Run(name, func(t *testing.T) {
			directory := t.TempDir()
			document, _ := largeEntryDocument(t, tail)
			if name == "trailing data" {
				document = append(document, []byte(`{}`)...)
			}
			legacy := filepath.Join(directory, "progress.json")
			if err := os.WriteFile(legacy, document, 0o600); err != nil {
				t.Fatal(err)
			}
			if store, err := Open(directory, false, entryImportConfig); err == nil || store != nil {
				t.Fatalf("invalid legacy document accepted: %v", err)
			}
			if _, err := os.Stat(filepath.Join(directory, Filename)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("database created after rejection: %v", err)
			}
			if retained, err := os.ReadFile(legacy); err != nil || !bytes.Equal(retained, document) {
				t.Fatalf("legacy state changed after rejection: %v", err)
			}
		})
	}
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), bytes.Repeat([]byte(" "), DocumentSizeLimit+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if store, err := Open(directory, false, entryImportConfig); err == nil || store != nil {
		t.Fatal("large whole document accepted")
	}
}
