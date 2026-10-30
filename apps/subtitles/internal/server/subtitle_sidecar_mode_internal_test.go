package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestSidecarWritesStayReadableByTheMediaServer(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(media, "Film.mkv")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(item.Path, 0o640); err != nil { //nolint:gosec // Media servers read sidecars through group or world permissions.
		t.Fatal(err)
	}
	provider := &subtitleProvider{index: sidecarTestIndex(item)}
	target, err := provider.openSidecar(item, "en")
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()

	if err = target.write("", []byte("first"), true); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(media, "Film.en.srt")
	if info, statErr := os.Stat(path); statErr != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("new sidecar mode = %v, %v; want the media file's 0640", info.Mode().Perm(), statErr)
	}
	if err = os.Chmod(path, 0o644); err != nil { //nolint:gosec // The household chose a world-readable sidecar.
		t.Fatal(err)
	}
	if err = target.write("", []byte("second"), false); err != nil {
		t.Fatal(err)
	}
	if info, statErr := os.Stat(path); statErr != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("replaced sidecar mode = %v, %v; want the existing 0644", info.Mode().Perm(), statErr)
	}
}

func TestSidecarWriteRemovesTemporaryFilesLeftByACrash(t *testing.T) {
	t.Parallel()
	media := t.TempDir()
	item := library.Item{ID: "0123456789abcdef", Kind: "video", Path: filepath.Join(media, "Film.mkv")}
	if err := os.WriteFile(item.Path, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale, fresh := filepath.Join(media, ".kinosail-subtitle-STALE"), filepath.Join(media, ".kinosail-subtitle-FRESH")
	for _, path := range []string{stale, fresh} {
		if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	provider := &subtitleProvider{index: sidecarTestIndex(item)}
	target, err := provider.openSidecar(item, "en")
	if err != nil {
		t.Fatal(err)
	}
	defer target.close()
	if err = target.write("", []byte("subtitle"), true); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Lstat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale temporary file remains: %v", err)
	}
	if _, err = os.Lstat(fresh); err != nil {
		t.Fatalf("a temporary file that may belong to a running write was removed: %v", err)
	}
}
