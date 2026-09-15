package privatefile_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/privatefile"
)

func TestWriteReadAndReplacePrivateFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")
	if err := privatefile.Write(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := privatefile.Write(path, []byte("second")); err != nil {
		t.Fatal(err)
	}
	data, err := privatefile.Read(path, 32)
	if err != nil || !bytes.Equal(data, []byte("second")) {
		t.Fatalf("read = %q, %v", data, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, %v", info, err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 || entries[0].Name() != "state.json" {
		t.Fatalf("entries = %#v, %v", entries, err)
	}
}

func TestWriteCacheReplacesPrivateFileWithoutDurabilitySync(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "entry")
	if err := privatefile.WriteCache(path, []byte("cached")); err != nil {
		t.Fatal(err)
	}
	data, err := privatefile.Read(path, 32)
	if err != nil || !bytes.Equal(data, []byte("cached")) {
		t.Fatalf("cache = %q, %v", data, err)
	}
}

func TestReadRejectsUnsafeFiles(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	if err := os.WriteFile(target, []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(directory, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(directory, "public")
	if err := os.WriteFile(public, []byte("public"), 0o644); err != nil { //nolint:gosec // The test needs a deliberately public file.
		t.Fatal(err)
	}
	for name, path := range map[string]string{"empty path": "", "missing": filepath.Join(directory, "missing"), "directory": directory, "symlink": link, "public": public, "oversized": target} {
		t.Run(name, func(t *testing.T) {
			maximum := int64(32)
			if name == "oversized" {
				maximum = 1
			}
			if _, err := privatefile.Read(path, maximum); err == nil {
				t.Fatalf("unsafe file %q was accepted", path)
			}
		})
	}
	if _, err := privatefile.Read(target, 0); err == nil {
		t.Fatal("zero read limit was accepted")
	}
}

func TestCreatePrivateMarkerOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "disabled")
	if err := privatefile.Create(path); err != nil {
		t.Fatal(err)
	}
	if err := privatefile.Create(path); !errors.Is(err, os.ErrExist) {
		t.Fatalf("second create error = %v", err)
	}
	data, err := privatefile.Read(path, 1)
	if err != nil || len(data) != 0 {
		t.Fatalf("marker = %q, %v", data, err)
	}
	if err := privatefile.Create(""); err == nil {
		t.Fatal("empty marker path was accepted")
	}
	if err := privatefile.Write("", nil); err == nil {
		t.Fatal("empty write path was accepted")
	}
}

func TestWriteAndCreateReportBlockedPaths(t *testing.T) {
	directory := t.TempDir()
	blocked := filepath.Join(directory, "blocked")
	if err := os.WriteFile(blocked, []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, operation := range map[string]func() error{
		"write":  func() error { return privatefile.Write(filepath.Join(blocked, "state"), nil) },
		"create": func() error { return privatefile.Create(filepath.Join(blocked, "marker")) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := operation(); err == nil {
				t.Fatal("blocked parent was accepted")
			}
		})
	}

	target := filepath.Join(directory, "existing-directory")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := privatefile.Write(target, []byte("replacement")); err == nil {
		t.Fatal("directory target was replaced")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("temporary file remains after failed write: %#v, %v", entries, err)
	}
}
