package backup_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/backup"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

// Isolated failure cases: tar overhead crosses the decoded limit; independently
// valid documents exceed the aggregate limit; incompressible state exceeds the
// encoded limit; encryption overhead crosses that same limit. Rejection must
// publish no bytes, and accepted archives must restore the persisted snapshot.
func TestPersistedBackupDecodedBoundaryRoundtrip(t *testing.T) {
	for _, test := range []struct {
		name, key string
		extra     int
	}{
		{"plain", "", 0},
		{"plain-over-limit", "", 1},
		{"encrypted", "synthetic roundtrip key", 0},
		{"encrypted-over-limit", "synthetic roundtrip key", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			dataDir := t.TempDir()
			documents := boundaryDocuments(test.extra)
			service := persistedService(t, dataDir, documents)
			var archive bytes.Buffer
			err := service.WriteAuto(&archive, dataDir, test.key, "dev")
			if test.extra != 0 {
				if err == nil || !strings.Contains(err.Error(), "maximum decoded size") || archive.Len() != 0 {
					t.Fatalf("over-limit write: error=%v output=%d", err, archive.Len())
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPersistedRoundtrip(t, service, archive.Bytes(), documents, test.key)
		})
	}
}

func assertPersistedRoundtrip(t *testing.T, service *backup.Service, archive []byte, documents map[string][]byte, key string) {
	t.Helper()
	if err := service.VerifyAuto(bytes.NewReader(archive), key); err != nil {
		t.Fatal(err)
	}
	restored := t.TempDir()
	if err := service.RestoreAuto(bytes.NewReader(archive), restored, key); err != nil {
		t.Fatal(err)
	}
	db, err := documentdb.Open(restored, false, documentdb.PlayerConfig())
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.Export()
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("restored export: %v, %v", err, closeErr)
	}
	for name, want := range documents {
		if !bytes.Equal(got[name], want) {
			t.Fatalf("restored %s differs", name)
		}
	}
}

func TestPersistedBackupRejectsAggregateAndEncodedOverflowBeforeOutput(t *testing.T) {
	for _, kind := range []string{"decoded", "encoded"} {
		t.Run(kind, func(t *testing.T) {
			dataDir := t.TempDir()
			documents := map[string][]byte{}
			names := []string{"collections.json", "history.json", "lists.json", "metadata.json", "progress.json"}
			payload := jsonPayload(14 << 20)
			if kind == "encoded" {
				payload = incompressibleJSON()
				names = names[:4]
			}
			for _, name := range names {
				documents[name] = payload
			}
			service := persistedService(t, dataDir, documents)
			for _, key := range []string{"", "synthetic roundtrip key"} {
				var archive bytes.Buffer
				if err := service.WriteAuto(&archive, dataDir, key, "dev"); err == nil || archive.Len() != 0 {
					t.Fatalf("overflow accepted: encrypted=%t error=%v output=%d", key != "", err, archive.Len())
				}
			}
		})
	}
}

func TestManagerPreservesRealRecoveryPointOnOversizedState(t *testing.T) {
	dataDir, directory := t.TempDir(), t.TempDir()
	service := persistedService(t, dataDir, map[string][]byte{"profiles.json": []byte(`{}`)})
	manager := backup.NewManager(backup.ManagerConfig{
		DataDir: dataDir, Directory: directory, Key: "synthetic roundtrip key", Retention: 1,
		WriteEncrypted: func(w io.Writer, dir, key string) error { return service.WriteEncrypted(w, dir, key, "dev") }, VerifyAuto: service.VerifyAuto,
	})
	if err := manager.WriteNow(); err != nil {
		t.Fatal(err)
	}
	before := manager.Status()
	saveOversizedSnapshot(t, dataDir)
	if err := manager.WriteNow(); err == nil {
		t.Fatal("oversized state accepted")
	}
	after := manager.Status()
	if after.Latest != before.Latest || after.LastSuccess != before.LastSuccess || after.LastVerified != before.LastVerified || after.LastError == "" {
		t.Fatalf("recovery status changed: before=%#v after=%#v", before, after)
	}
	assertRetainedRecoveryPoint(t, manager, service)
}

func assertRetainedRecoveryPoint(t *testing.T, manager *backup.Manager, service *backup.Service) {
	t.Helper()
	paths, err := manager.Files()
	if err != nil || len(paths) != 1 {
		t.Fatalf("retained backups: %v, %v", paths, err)
	}
	file, err := os.Open(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := service.VerifyAuto(file, "synthetic roundtrip key"); err != nil {
		t.Fatal(err)
	}
}

func persistedService(t *testing.T, directory string, documents map[string][]byte) *backup.Service {
	t.Helper()
	for name, data := range documents {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	db, err := documentdb.Open(directory, false, documentdb.PlayerConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return backup.MustNew(backup.Config{DatabaseFilename: "kinosail.db", OpenDatabase: func(dir string) (backup.Database, error) {
		return documentdb.Open(dir, false, documentdb.PlayerConfig())
	}, ValidateSettings: func([]byte) error { return nil }})
}

func jsonPayload(size int) []byte {
	return []byte(`{"value":"` + strings.Repeat("x", size-len(`{"value":""}`)) + `"}`)
}

func saveOversizedSnapshot(t *testing.T, dataDir string) {
	t.Helper()
	db, err := documentdb.Open(dataDir, false, documentdb.PlayerConfig())
	if err != nil {
		t.Fatal(err)
	}
	documents := map[string][]byte{}
	for _, name := range []string{"collections.json", "history.json", "lists.json", "metadata.json", "progress.json"} {
		documents[name] = jsonPayload(14 << 20)
	}
	values := map[string]any{}
	for name, data := range documents {
		values[name] = json.RawMessage(data)
	}
	if err := db.SaveJSONBatch(values); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func incompressibleJSON() []byte {
	raw := make([]byte, 11<<20)
	// This fixed PRNG seed is test data, never an encryption key or nonce.
	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // Reproducible compression fixture, not cryptography.
	for i := range raw {
		raw[i] = byte(rng.Uint32() & 255)
	}
	return []byte(`{"value":"` + base64.StdEncoding.EncodeToString(raw) + `"}`)
}

func boundaryDocuments(extra int) map[string][]byte {
	documents := map[string][]byte{}
	for _, name := range []string{"collections.json", "history.json", "lists.json", "metadata.json"} {
		size := (16 << 20) - 1024
		if name == "metadata.json" {
			size += extra
		}
		documents[name] = jsonPayload(size)
	}
	return documents
}
