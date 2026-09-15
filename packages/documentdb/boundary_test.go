package documentdb

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRejectedSaveBoundariesKeepPriorDocuments(t *testing.T) { //nolint:cyclop,gocognit // Scores of 16 and 20 remain below the repository ceiling of 22 for the rejection matrix.
	config := testConfig
	config.Validate = func(_ string, data []byte) error {
		if bytes.Contains(data, []byte(`"blocked":true`)) {
			return errors.New("blocked document")
		}
		return nil
	}
	store, err := Open(t.TempDir(), false, config)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	want := map[string]string{"name": "Before"}
	if err := store.SaveJSON("settings.json", want); err != nil {
		t.Fatal(err)
	}
	atLimit := json.RawMessage(`"` + strings.Repeat("x", DocumentSizeLimit-2) + `"`)
	if err := store.SaveJSON("settings.json", atLimit); err != nil {
		t.Fatalf("exact size limit was rejected: %v", err)
	}
	if data, found, err := store.Load("settings.json"); err != nil || !found || len(data) != DocumentSizeLimit {
		t.Fatalf("exact size document = %d bytes, found=%t, error=%v", len(data), found, err)
	}
	if err := store.SaveJSON("settings.json", want); err != nil {
		t.Fatal(err)
	}

	for name, operation := range map[string]func() error{
		"oversized": func() error {
			return store.SaveJSON("settings.json", json.RawMessage(`"`+strings.Repeat("x", DocumentSizeLimit-1)+`"`))
		},
		"semantic": func() error {
			return store.SaveJSON("settings.json", map[string]bool{"blocked": true})
		},
		"over cardinality": func() error {
			return store.SaveJSONBatch(map[string]any{"settings.json": true, "profiles.json": true, "extra.json": true})
		},
		"canceled": func() error {
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			return store.SaveJSONBatchContext(ctx, map[string]any{"settings.json": map[string]string{"name": "After"}})
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); err == nil {
				t.Fatal("rejected save was accepted")
			}
			var got map[string]string
			if found, err := store.LoadJSON("settings.json", &got); err != nil || !found || !reflect.DeepEqual(got, want) {
				t.Fatalf("prior document = %#v, found=%t, error=%v", got, found, err)
			}
			exported, err := store.Export()
			if err != nil || len(exported) != 1 {
				t.Fatalf("rejected save changed document set: %#v, %v", exported, err)
			}
		})
	}
}

func writeOversizedLegacy(path string) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := file.Truncate(DocumentSizeLimit + 1); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func writeSymlinkLegacy(path string) error {
	target := filepath.Join(filepath.Dir(path), "outside")
	if err := os.WriteFile(target, []byte(`{}`), 0o600); err != nil {
		return err
	}
	return os.Symlink(target, path)
}
