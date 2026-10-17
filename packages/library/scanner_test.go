package library_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func writeScannerFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func scannedTitles(items []library.Item) map[string]string {
	titles := make(map[string]string, len(items))
	for _, item := range items {
		titles[filepath.Base(item.Path)] = item.Title + "|" + item.ShowArtwork
	}
	return titles
}

func TestScannerRescansOnlyChangedDirectories(t *testing.T) {
	root := t.TempDir()
	season := filepath.Join(root, "Show", "Season 01")
	writeScannerFile(t, filepath.Join(season, "Show S01E01.mkv"), "media")
	writeScannerFile(t, filepath.Join(root, "Film (2020).mp4"), "media")
	scanner := library.NewScanner(root, "Library")
	if scanner.Root() != root {
		t.Fatalf("root = %q", scanner.Root())
	}
	items, err := scanner.Scan(t.Context())
	if err != nil || len(items) != 2 {
		t.Fatalf("scan = %#v, %v", items, err)
	}
	if changed := scanner.Changed(t.Context()); len(changed) != 0 {
		t.Fatalf("unchanged directories reported = %#v", changed)
	}
	if unchanged, err := scanner.Rescan(t.Context(), []string{filepath.Join(t.TempDir(), "outside.mkv")}); err != nil || len(unchanged) != 2 {
		t.Fatalf("outside change = %#v, %v", unchanged, err)
	}
	time.Sleep(10 * time.Millisecond)
	writeScannerFile(t, filepath.Join(root, "Show", "poster.jpg"), "poster")
	extras := filepath.Join(root, "Extras", "Nested")
	writeScannerFile(t, filepath.Join(extras, "Clip.mp4"), "media")
	if err := os.Remove(filepath.Join(root, "Film (2020).mp4")); err != nil {
		t.Fatal(err)
	}
	if changed := scanner.Changed(t.Context()); len(changed) == 0 {
		t.Fatal("changed directories were not reported")
	}
	items, err = scanner.Rescan(t.Context(), []string{filepath.Join(root, "Show", "poster.jpg"), filepath.Join(extras, "Clip.mp4"), filepath.Join(root, "Film (2020).mp4")})
	titles := scannedTitles(items)
	if err != nil || len(items) != 2 || titles["Show S01E01.mkv"] != "S01E01|"+filepath.Join(root, "Show", "poster.jpg") || titles["Clip.mp4"] == "" {
		t.Fatalf("rescan = %#v, %v", titles, err)
	}
	if err := os.RemoveAll(filepath.Join(root, "Show")); err != nil {
		t.Fatal(err)
	}
	items, err = scanner.Rescan(t.Context(), []string{season})
	if err != nil || len(items) != 1 || items[0].Title != "Clip" {
		t.Fatalf("removed directory rescan = %#v, %v", items, err)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	if _, err := scanner.Rescan(t.Context(), []string{root}); err == nil {
		t.Fatal("missing root was accepted")
	}
}

func TestScannerHandlesFileAndSymlinkRoots(t *testing.T) {
	directory := t.TempDir()
	media := filepath.Join(directory, "Song.flac")
	writeScannerFile(t, media, "audio")
	items, err := library.NewScanner(media, "Music").Rescan(t.Context(), []string{media})
	if err != nil || len(items) != 1 || items[0].Kind != "audio" || items[0].Path != media {
		t.Fatalf("file root = %#v, %v", items, err)
	}
	link := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(directory, link); err != nil {
		t.Fatal(err)
	}
	if items, err := library.ScanContext(t.Context(), link, "Music"); err != nil || len(items) != 0 {
		t.Fatalf("symlinked root = %#v, %v", items, err)
	}
	if items, err := library.ScanContext(t.Context(), "", "Music"); err != nil || items != nil {
		t.Fatalf("empty root = %#v, %v", items, err)
	}
}

func TestScannerReportsChangesOnlyWithinRequestedSubtrees(t *testing.T) {
	root := t.TempDir()
	local, remote, sibling := filepath.Join(root, "Local"), filepath.Join(root, "Remote"), filepath.Join(root, "Remote Copy")
	nested := filepath.Join(remote, "Nested")
	for _, directory := range []string{local, nested, sibling} {
		writeScannerFile(t, filepath.Join(directory, "Film (2020).mp4"), "media")
	}
	scanner := library.NewScanner(root, "Library")
	if _, err := scanner.Scan(t.Context()); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	for _, directory := range []string{local, remote, nested, sibling} {
		writeScannerFile(t, filepath.Join(directory, "Added (2021).mp4"), "media")
	}
	changed := scanner.Changed(t.Context(), remote)
	if _, found := changed[remote]; !found || len(changed) != 2 {
		t.Fatalf("changes within the remote subtree = %#v", changed)
	}
	if _, found := changed[nested]; !found {
		t.Fatalf("nested remote change missing: %#v", changed)
	}
	if all := scanner.Changed(t.Context()); len(all) != 4 {
		t.Fatalf("changes across the root = %#v", all)
	}
}
