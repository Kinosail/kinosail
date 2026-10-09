package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func damageCopiedAACPartial(t *testing.T, manager *hlsManager, source, directory, damage string) {
	t.Helper()
	writes := map[string]struct{ name, data string }{
		"init":             {"360p/init.mp4", "invalid init"},
		"binding":          {".source", "wrong binding"},
		"timeline":         {".copy-timeline", "{}"},
		"certificate":      {".copy-clock", "{}"},
		"master":           {"index.m3u8", "#EXTM3U\n"},
		"startup-value":    {".startup", "0"},
		"startup-empty":    {".startup", ""},
		"startup-overflow": {".startup", "11"},
	}
	if value, ok := writes[damage]; ok {
		writeHLSLoadingFile(t, filepath.Join(directory, value.name), value.data)
		return
	}
	if damage == "source-root" {
		manager.index.SetRoots(nil)
		return
	}
	if damage == "first-cut" {
		if err := os.Remove(filepath.Join(directory, "360p/segment-00000.m4s")); err != nil {
			t.Fatal(err)
		}
		return
	}
	damageCopiedAACPartialKind(t, source, directory, damage)
}

func damageCopiedAACPartialKind(t *testing.T, source, directory, damage string) {
	t.Helper()
	if damage != "startup-directory" && damage != "startup-symlink" && damage != "later-directory" && damage != "later-symlink" {
		t.Fatal("unknown partial damage kind")
	}
	path := filepath.Join(directory, "360p/segment-00001.m4s")
	target := filepath.Join(directory, "360p/segment-00000.m4s")
	if strings.HasPrefix(damage, "startup-") {
		path, target = filepath.Join(directory, ".startup"), source
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if strings.HasSuffix(damage, "-directory") {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}
}
