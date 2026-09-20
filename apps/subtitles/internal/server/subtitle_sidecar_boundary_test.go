package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSubtitleSidecarKeepsAuthorizedParentAfterRebind(t *testing.T) {
	libraryPath, outside := t.TempDir(), t.TempDir()
	parent := filepath.Join(libraryPath, "film")
	if err := os.Mkdir(parent, 0700); err != nil {
		t.Fatal(err)
	}
	item := library.Item{ID: "film", Kind: "video", Path: filepath.Join(parent, "film.mp4")}
	index := sidecarTestIndex(item)
	index.SetRoots([]libraryRoot{{Path: libraryPath}})
	provider := &subtitleProvider{index: index}
	target, err := provider.openSidecar(item, "en")
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()
	moved := filepath.Join(libraryPath, "original")
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, parent); err != nil {
		t.Fatal(err)
	}
	if err := target.write("", []byte("original"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(outside, target.name)); !os.IsNotExist(err) {
		t.Fatal("write escaped Library")
	}
	if data, err := os.ReadFile(filepath.Join(moved, target.name)); err != nil || string(data) != "original" {
		t.Fatal("original parent was not retained")
	}
	if _, err := provider.openSidecar(item, "en"); err == nil {
		t.Fatal("escaped parent accepted")
	}
	if err := target.remove(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moved, target.name)); !os.IsNotExist(err) {
		t.Fatal("rollback missed retained parent")
	}
}
