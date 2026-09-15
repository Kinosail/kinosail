package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestArchiveRoundTrip(t *testing.T) { //nolint:cyclop // One round trip checks each expected and excluded state file.
	t.Parallel()

	source, restored := t.TempDir(), t.TempDir()
	want := map[string]string{
		"audit.jsonl":          "{\"time\":\"2026-08-23T00:00:00Z\",\"action\":\"owner.created\"}\n",
		"configuration.json":   `{"backup.retention":"12"}`,
		"lists.json":           `{"owner:item":true}`,
		"playlists.json":       `{"owner:Favorites":{"item":true}}`,
		"profiles.json":        `{"profiles":[]}`,
		"progress.json":        `{"item":{"seconds":42}}`,
		"sessions.json":        `{"token":"viewer"}`,
		"settings.json":        `{"name":"Living Room","libraries":["."]}`,
		"server-identity.json": `{"seed":"offline-recovery-copy","sequence":42}`,
	}
	for name, content := range want {
		if err := os.WriteFile(filepath.Join(source, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "not-backed-up.txt"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "viewing_imports.json"), []byte(`{"sync":{"token":"private"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := testArchive.Write(&archive, source); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.Restore(bytes.NewReader(archive.Bytes()), restored); err != nil {
		t.Fatal(err)
	}
	for name, expected := range want {
		actual, err := os.ReadFile(filepath.Join(restored, name))
		if err != nil || string(actual) != expected {
			t.Fatalf("%s = %q, %v", name, actual, err)
		}
	}
	if _, err := os.Stat(filepath.Join(restored, "not-backed-up.txt")); !os.IsNotExist(err) {
		t.Fatalf("unexpected file restored: %v", err)
	}
	if _, err := os.Stat(filepath.Join(restored, "viewing_imports.json")); !os.IsNotExist(err) {
		t.Fatalf("viewing sync secret was included in a plain backup: %v", err)
	}
}

func TestArchiveContainsRecoveryManifest(t *testing.T) { //nolint:cyclop // One archive fixture validates every recovery-manifest invariant.
	t.Parallel()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "settings.json"), []byte(`{"name":"Home"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := testArchive.Write(&archive, directory); err != nil {
		t.Fatal(err)
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	tarReader := tar.NewReader(gzipReader)
	var description struct {
		Format          int    `json:"format"`
		KinosailVersion string `json:"kinosailVersion"`
		StateSchema     int    `json:"stateSchema"`
	}
	for {
		header, nextErr := tarReader.Next()
		if nextErr != nil {
			break
		}
		if header.Name == "manifest.json" {
			if err := json.NewDecoder(tarReader).Decode(&description); err != nil {
				t.Fatal(err)
			}
		}
	}
	if description.Format != 1 || description.KinosailVersion == "" || description.StateSchema != 1 {
		t.Fatalf("invalid backup manifest: %+v", description)
	}
}

func TestRestoreRejectsTraversal(t *testing.T) {
	t.Parallel()

	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte(`{}`)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../profiles.json", Mode: 0o600, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tarWriter.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.Restore(bytes.NewReader(archive.Bytes()), t.TempDir()); err == nil {
		t.Fatal("traversal archive was accepted")
	}
}

func TestRestoreRemovesManagedStateAbsentFromSnapshot(t *testing.T) {
	t.Parallel()
	source, restored := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "profiles.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(restored, "settings.json"), []byte(`{"name":"changed"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	if err := testArchive.Write(&archive, source); err != nil {
		t.Fatal(err)
	}
	if err := testArchive.Restore(bytes.NewReader(archive.Bytes()), restored); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(restored, "settings.json")); !os.IsNotExist(err) {
		t.Fatalf("newer state survived restore: %v", err)
	}
}
