package server

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

const hotManifestOriginal = "#EXTM3U\n#EXT-X-TARGETDURATION:4\n#EXTINF:4,\nsegment-00000.m4s\n"

// Public HTTP cannot deterministically stop a regular-file read after open.
// This owns a real descriptor across the exact atomic publication window;
// transport/admission is covered separately by the registered HTTP regression.
func TestHLSHotManifestSnapshotSurvivesAtomicPublication(t *testing.T) {
	root := hotManifestRoot(t, []byte(hotManifestOriginal))
	file, before, err := copiedHLSOpenFile(root, "index.m3u8", 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	replacement := []byte(hotManifestOriginal + "#EXTINF:4,\nsegment-00001.m4s\n")
	if err := root.WriteFile("next.m3u8", replacement, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.Rename("next.m3u8", "index.m3u8"); err != nil {
		t.Fatal(err)
	}
	current, err := root.Lstat("index.m3u8")
	if err != nil || os.SameFile(before, current) {
		t.Fatal("publisher did not replace the pathname")
	}
	data, err := readHLSManifestFile(root, file, before)
	if err != nil || !bytes.Equal(data, []byte(hotManifestOriginal)) {
		t.Fatal("complete opened snapshot rejected after atomic publication")
	}
	data, err = readHLSManifest(root)
	if err != nil || !bytes.Equal(data, replacement) {
		t.Fatal("next read did not observe committed replacement")
	}
}

func TestHLSHotManifestSnapshotRejectsUnsafeReplacement(t *testing.T) {
	for _, change := range []string{"in-place append", "in-place truncate", "in-place timestamp", "missing", "symlink", "directory", "empty", "oversized"} {
		t.Run(change, func(t *testing.T) {
			root := hotManifestRoot(t, []byte(hotManifestOriginal))
			file, before, err := copiedHLSOpenFile(root, "index.m3u8", 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			changeHotManifest(t, root, change)
			data, err := readHLSManifestFile(root, file, before)
			if !errors.Is(err, errCopiedHLSIndex) || len(data) != 0 {
				t.Fatal("unsafe snapshot admitted")
			}
			protected, err := root.ReadFile("protected")
			if err != nil || string(protected) != "keep" {
				t.Fatal("reader changed unrelated data")
			}
		})
	}
}

func TestHLSHotManifestOpenRejectsUnsafeFile(t *testing.T) {
	for _, change := range []string{"missing", "symlink", "outside symlink", "directory", "empty", "oversized", "fifo"} {
		t.Run(change, func(t *testing.T) {
			root := hotManifestRoot(t, []byte(hotManifestOriginal))
			changeHotManifest(t, root, change)
			assertHotManifestUnsafeOpen(t, root, change)
		})
	}
}

func assertHotManifestUnsafeOpen(t *testing.T, root *os.Root, change string) {
	t.Helper()
	done := make(chan error, 1)
	go func() { _, err := readHLSManifest(root); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("unsafe manifest opened")
		}
	case <-time.After(time.Second):
		// Release the owned FIFO and join if nonblocking admission regresses.
		if change == "fifo" {
			releaseHotManifestFIFO(t, root)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("reader did not finish after owned FIFO release")
		}
		t.Fatal("manifest read blocked")
	}
}

func hotManifestRoot(t *testing.T, data []byte) *os.Root {
	t.Helper()
	root, err := os.OpenRoot(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := root.WriteFile("index.m3u8", data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("protected", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	return root
}

func changeHotManifest(t *testing.T, root *os.Root, change string) {
	t.Helper()
	if strings.HasPrefix(change, "in-place") {
		changeHotManifestInPlace(t, root, change)
		return
	}
	if err := root.Remove("index.m3u8"); err != nil {
		t.Fatal(err)
	}
	if change == "outside symlink" {
		outside := hotManifestRoot(t, []byte("outside keep"))
		if err := root.Symlink(outside.Name()+"/protected", "index.m3u8"); err != nil {
			t.Fatal(err)
		}
		return
	}
	if err := replaceHotManifest(root, change); err != nil {
		t.Fatal(err)
	}
}

func replaceHotManifest(root *os.Root, change string) error {
	switch change {
	case "missing":
		return nil
	case "symlink":
		return root.Symlink("protected", "index.m3u8")
	case "directory":
		return root.Mkdir("index.m3u8", 0o700)
	case "empty":
		return root.WriteFile("index.m3u8", nil, 0o600)
	case "oversized":
		return root.WriteFile("index.m3u8", []byte(strings.Repeat("x", (1<<20)+1)), 0o600)
	case "fifo":
		return syscall.Mkfifo(root.Name()+"/index.m3u8", 0o600)
	default:
		return errors.New("unknown mutation")
	}
}

func changeHotManifestInPlace(t *testing.T, root *os.Root, change string) {
	t.Helper()
	if change == "in-place timestamp" {
		info, err := root.Stat("index.m3u8")
		if err != nil {
			t.Fatal(err)
		}
		if err := root.Chtimes("index.m3u8", info.ModTime().Add(time.Second), info.ModTime().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		return
	}
	data := []byte("x")
	if change == "in-place append" {
		data = []byte(hotManifestOriginal + "x")
	}
	if err := root.WriteFile("index.m3u8", data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func releaseHotManifestFIFO(t *testing.T, root *os.Root) {
	t.Helper()
	file, err := root.OpenFile("index.m3u8", os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}
