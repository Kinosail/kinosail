package server

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Public media cannot inject untrusted metadata or an interrupted two-file commit.
func TestCopiedRecoveryCertificateDamageRejectsWithoutWrites(t *testing.T) {
	for _, damage := range []string{"missing", "malformed", "duplicate", "oversized", "symlink", "fifo"} {
		t.Run(damage, func(t *testing.T) {
			manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
			copiedRecoveryProbe(t, manager, "")
			if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err != nil {
				t.Fatal(err)
			}
			mapPath := filepath.Join(directory, ".copy-timeline")
			before, err := os.ReadFile(mapPath)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(directory, ".copy-clock")
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			outside := filepath.Join(t.TempDir(), "outside")
			writeHLSLoadingFile(t, outside, "protected")
			switch damage {
			case "malformed":
				writeHLSLoadingFile(t, path, "{")
			case "duplicate":
				writeHLSLoadingFile(t, path, `{"version":1,"version":1}`)
			case "oversized":
				writeHLSLoadingFile(t, path, strings.Repeat("x", 4097))
			case "symlink":
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(path, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if manager.reusableCopiedHLS(t.Context(), item, directory, policy, recipe) {
				t.Error("untrusted or interrupted clock certificate remained reusable")
			}
			if got := manager.copiedPlaylistProjection(t.Context(), item, recipe, directory, "360p", policy)([]byte(copiedRecoveryManifest)); len(got) != 0 {
				t.Error("damaged certificate exposed a playlist")
			}
			after, err := os.ReadFile(mapPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("rejection changed the timeline")
			}
			if data, err := os.ReadFile(outside); err != nil || string(data) != "protected" {
				t.Fatal("rejection changed an external file")
			}
		})
	}
}

func TestCopiedRecoveryProbeCannotOverwriteChangedTimeline(t *testing.T) {
	manager, item, recipe, directory, policy, timeline := copiedRecoveryFixture(t)
	path := filepath.Join(directory, ".copy-timeline")
	copiedRecoveryProbe(t, manager, "cp "+copiedRecoveryQuote(path)+" "+copiedRecoveryQuote(path+".replacement")+"\nmv "+copiedRecoveryQuote(path+".replacement")+" "+copiedRecoveryQuote(path))
	if err := manager.bindCopiedHLSClock(t.Context(), item, recipe, directory, "360p/index.m3u8", policy, timeline); err == nil {
		t.Fatal("probe certified a replacement timeline inode")
	}
}
