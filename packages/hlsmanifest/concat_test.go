package hlsmanifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/playback"
)

func TestWriteSkipConcatWritesOnlyRetainedRanges(t *testing.T) {
	directory := t.TempDir()
	path, err := WriteSkipConcat(directory, "/media/O'Brien.mkv", 40, []playback.Range{{Start: 0, End: 5}, {Start: 10, End: 20}, {Start: 30, End: 30}})
	if err != nil || path != filepath.Join(directory, "skip.ffconcat") {
		t.Fatalf("path = %q, error = %v", path, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "ffconcat version 1.0\nfile '/media/O'\\''Brien.mkv'\ninpoint 5.000000\noutpoint 10.000000\nfile '/media/O'\\''Brien.mkv'\ninpoint 20.000000\noutpoint 30.000000\nfile '/media/O'\\''Brien.mkv'\ninpoint 30.000000\noutpoint 40.000000\n"
	if string(data) != want {
		t.Fatalf("manifest = %q, want %q", data, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", info.Mode().Perm())
	}
}

func TestWriteSkipConcatWritesInitialSpanWithoutInpoint(t *testing.T) {
	path, err := WriteSkipConcat(t.TempDir(), "/media/movie.mkv", 5, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(data); got != "ffconcat version 1.0\nfile '/media/movie.mkv'\noutpoint 5.000000\n" || strings.Contains(got, "inpoint") {
		t.Fatalf("manifest = %q", got)
	}
}

func TestWriteSkipConcatRejectsUnsafeSourcesBeforeWriting(t *testing.T) {
	for _, source := range []string{"/media/line\nbreak.mkv", "/media/line\rbreak.mkv"} {
		directory := t.TempDir()
		path, err := WriteSkipConcat(directory, source, 10, nil)
		if err == nil || path != "" {
			t.Fatalf("source %q returned path %q, error %v", source, path, err)
		}
		if _, statErr := os.Stat(filepath.Join(directory, "skip.ffconcat")); !os.IsNotExist(statErr) {
			t.Fatalf("source %q wrote a manifest: %v", source, statErr)
		}
	}
}

func TestWriteSkipConcatReturnsStorageFailure(t *testing.T) {
	path, err := WriteSkipConcat(filepath.Join(t.TempDir(), "missing"), "/media/movie.mkv", 0, nil)
	if err == nil || !strings.HasSuffix(path, "skip.ffconcat") {
		t.Fatalf("path = %q, error = %v", path, err)
	}
}
