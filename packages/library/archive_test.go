package library

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/MikeO7/kinosail/packages/archivetest"
)

func TestArchiveImagesAndZIPAssets(t *testing.T) { //nolint:cyclop // One fixture protects archive listing, bounds, traversal, and corruption behavior.
	archivePath := filepath.Join(t.TempDir(), "Comic.cbz")
	writeArchiveZIP(t, archivePath, map[string]string{
		"folder/a.jpg": "one", "b.WEBP": "two", "notes.txt": "ignored", "../escape.png": "unsafe",
	})
	item := Item{Path: archivePath, Container: "CBZ"}
	if images := ArchiveImages(t.Context(), item); !slices.Equal(images, []ArchiveImage{{Name: "folder/a.jpg", Title: "a.jpg"}, {Name: "b.WEBP", Title: "b.WEBP"}}) {
		t.Fatalf("archive images = %#v", images)
	}
	data, err := ReadArchiveAsset(t.Context(), item, "folder/a.jpg")
	if err != nil || string(data) != "one" {
		t.Fatalf("archive asset = %q, %v", data, err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if data, err = ReadArchive(archive.File, "folder/a.jpg", 2); err == nil || data != nil || err.Error() != "archive entry not found" {
		t.Fatalf("oversized archive asset = %q, %v", data, err)
	}
	_ = archive.Close()
	for _, name := range []string{"", "../escape.png", "/absolute.png", `folder\image.png`, "missing.jpg"} {
		if data, err = ReadArchiveAsset(t.Context(), item, name); err == nil || data != nil || err.Error() != "archive entry not found" {
			t.Fatalf("invalid asset %q = %q, %v", name, data, err)
		}
	}
	bad := filepath.Join(t.TempDir(), "Broken.cbz")
	if err := os.WriteFile(bad, []byte("not a zip"), 0o600); err != nil {
		t.Fatal(err)
	}
	if images := ArchiveImages(t.Context(), Item{Path: bad, Container: "CBZ"}); len(images) != 0 {
		t.Fatalf("broken archive images = %#v", images)
	}
	if _, err := ReadArchiveAsset(t.Context(), Item{Path: bad, Container: "CBZ"}, "a.jpg"); err == nil {
		t.Fatal("broken archive asset was read")
	}
}

func TestArchiveEntryBoundsAndErrors(t *testing.T) { //nolint:cyclop // One bounded archive fixture covers every entry-limit failure.
	root := t.TempDir()
	archivePath := filepath.Join(root, "Bounded.cbz")
	files := make(map[string]string, archiveEntryLimit+1)
	for index := range archiveEntryLimit + 1 {
		files[fmt.Sprintf("pages/%05d.jpg", index)] = ""
	}
	writeArchiveZIP(t, archivePath, files)
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if entries := archiveEntries(t.Context(), Item{Path: archivePath, Container: "CBZ"}); len(entries) != archiveEntryLimit {
		t.Fatalf("archive entry count = %d", len(entries))
	}
	if data, err := ReadArchive(archive.File, "", 1); err == nil || data != nil || err.Error() != "archive entry not found" {
		t.Fatalf("invalid archive entry = %q, %v", data, err)
	}
	if data, err := ReadArchive(archive.File, archive.File[0].Name, 0); err == nil || data != nil {
		t.Fatalf("invalid archive limit = %q, %v", data, err)
	}
	if data, err := ReadArchive(archive.File, archive.File[0].Name, -1); err == nil || data != nil {
		t.Fatalf("negative archive limit = %q, %v", data, err)
	}
}

func TestTarArchiveImagesAndAssets(t *testing.T) { //nolint:cyclop // One fixture protects tar listing, traversal, and corruption behavior.
	if _, err := exec.LookPath("bsdtar"); err != nil {
		t.Fatalf("bsdtar is required: %v", err)
	}
	root := t.TempDir()
	archivePath := filepath.Join(root, "Comic.cbt")
	writeArchiveTar(t, archivePath, map[string]string{"-page.jpg": "first", "002.png": "second", "notes.txt": "ignored"})
	item := Item{Path: archivePath, Container: "cBt"}
	if images := ArchiveImages(t.Context(), item); !slices.Equal(images, []ArchiveImage{{Name: "-page.jpg", Title: "-page.jpg"}, {Name: "002.png", Title: "002.png"}}) {
		t.Fatalf("tar images = %#v", images)
	}
	data, err := ReadArchiveAsset(t.Context(), item, "-page.jpg")
	if err != nil || string(data) != "first" {
		t.Fatalf("tar asset = %q, %v", data, err)
	}
	if data, err = ReadArchiveAsset(t.Context(), item, "missing.jpg"); err == nil || data != nil || err.Error() != "archive entry not found" {
		t.Fatalf("missing tar asset = %q, %v", data, err)
	}
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if data, err = ReadArchiveAsset(t.Context(), item, "../escape.jpg"); err == nil || data != nil {
		t.Fatalf("unsafe tar asset = %q, %v", data, err)
	}
	after, err := os.ReadDir(root)
	if err != nil || len(after) != len(before) {
		t.Fatalf("rejected archive changed storage: before=%d after=%d error=%v", len(before), len(after), err)
	}
	broken := filepath.Join(root, "Broken.cbt")
	if err := os.WriteFile(broken, []byte("not an archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if images := ArchiveImages(t.Context(), Item{Path: broken, Container: "CBT"}); len(images) != 0 {
		t.Fatalf("broken tar images = %#v", images)
	}
}

func TestBoundedArchiveCommand(t *testing.T) { //nolint:cyclop // One matrix protects every process and output boundary.
	if data, err := boundedCommand(t.Context(), "printf", 4, "%s", "four"); err != nil || string(data) != "four" {
		t.Fatalf("bounded output = %q, %v", data, err)
	}
	if data, err := boundedCommand(t.Context(), "printf", 4, "%s", "oversized"); err == nil || data != nil || err.Error() != "archive output is invalid" {
		t.Fatalf("oversized output = %q, %v", data, err)
	}
	if data, err := boundedCommand(t.Context(), "false", 4); err == nil || data != nil || err.Error() != "archive could not be read" {
		t.Fatalf("failed command output = %q, %v", data, err)
	}
	for _, test := range []struct {
		ctx        context.Context
		executable string
		limit      int64
	}{
		{nil, "printf", 4}, {t.Context(), "", 4}, {t.Context(), "printf", 0}, {t.Context(), "missing-kinosail-archive-command", 4},
	} {
		if data, err := boundedCommand(test.ctx, test.executable, test.limit); err == nil || data != nil || err.Error() != "archive reader unavailable" {
			t.Fatalf("invalid command = %q, %v", data, err)
		}
	}
	if data, err := boundedOutput(errorArchiveReader{}, 4); err == nil || data != nil || err.Error() != "archive output is invalid" {
		t.Fatalf("failed read = %q, %v", data, err)
	}
}

func TestReadArchivePropagatesUnsupportedCompression(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "Unsupported.cbz")
	writeArchiveZIP(t, archivePath, map[string]string{"page.jpg": "page"})
	data, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []struct {
		signature []byte
		offset    int
	}{{[]byte("PK\x03\x04"), 8}, {[]byte("PK\x01\x02"), 10}} {
		index := bytes.Index(data, marker.signature)
		if index < 0 {
			t.Fatal("ZIP header is missing")
		}
		data[index+marker.offset], data[index+marker.offset+1] = 99, 0
	}
	if err := os.WriteFile(archivePath, data, 0o600); err != nil { //nolint:gosec // The path is created inside this test's temporary directory.
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if data, err := ReadArchive(archive.File, "page.jpg", 64); !errors.Is(err, zip.ErrAlgorithm) || data != nil {
		t.Fatalf("unsupported compression = %q, %v", data, err)
	}
}

type errorArchiveReader struct{}

func (errorArchiveReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

func writeArchiveZIP(t *testing.T, path string, files map[string]string) {
	t.Helper()
	archivetest.WriteZIP(t, path, files)
}

func writeArchiveTar(t *testing.T, path string, files map[string]string) {
	t.Helper()
	archivetest.WriteTar(t, path, files)
}
