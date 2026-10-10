package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func damageCopiedAACPartialFence(t *testing.T, manager *hlsManager, item library.Item, directory, damage string) {
	t.Helper()
	if strings.HasPrefix(damage, "startup-") {
		damageCopiedAACPartialStartup(t, directory, damage)
		return
	}
	if damage == "later-arrived" {
		writeHLSLoadingFile(t, filepath.Join(directory, "360p/segment-00001.m4s"), "foreign later cut")
		return
	}
	if damage == "later-directory" || damage == "later-symlink" {
		damageCopiedAACPartialKind(t, item.Path, directory, damage)
		return
	}
	if damage == "later-remove" || damage == "later-replace" {
		damageCopiedAACPartialLater(t, directory, damage)
		return
	}
	known := map[string]bool{"source": true, "source-root": true, "generation": true, "rendition": true}
	if !known[damage] {
		t.Fatal("unknown partial fence damage")
	}
	damageCopiedAACLegacy(t, manager, item, directory, damage)
}

func damageCopiedAACPartialStartup(t *testing.T, directory, damage string) {
	t.Helper()
	path := filepath.Join(directory, ".startup")
	switch damage {
	case "startup-added":
		writeHLSLoadingFile(t, path, "1")
	case "startup-removed":
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	case "startup-replaced":
		replaceCopiedAACLegacyMetadata(t, path)
	case "startup-same-stat-value":
		before, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		writeHLSLoadingFile(t, path, "0")
		if err := os.Chtimes(path, before.ModTime(), before.ModTime()); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatal("unknown partial startup damage")
	}
}

func damageCopiedAACPartialLater(t *testing.T, directory, damage string) {
	t.Helper()
	path := filepath.Join(directory, "360p/segment-00001.m4s")
	if damage == "later-replace" {
		replaceCopiedAACLegacyMetadata(t, path)
		return
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
}
