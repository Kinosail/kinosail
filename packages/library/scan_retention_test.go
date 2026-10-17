package library_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func retentionLibrary(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	probe := filepath.Join(root, "probe")
	if err := os.Mkdir(probe, 0); err != nil {
		t.Fatal(err)
	}
	if entries, err := os.ReadDir(probe); err == nil {
		t.Skipf("directory permissions are not enforced for this user (%d entries)", len(entries))
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(root, "Collection")
	writeScannerFile(t, filepath.Join(root, "Visible (2020).mp4"), "media")
	writeScannerFile(t, filepath.Join(folder, "Kept (2021).mp4"), "media")
	writeScannerFile(t, filepath.Join(folder, "Nested", "Deep (2022).mp4"), "media")
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) }) //nolint:gosec // Cleanup restores owner-only directory traversal in the test sandbox.
	return root, folder
}

func sortedTitles(items []library.Item, err error) []string {
	if err != nil {
		return []string{"error: " + err.Error()}
	}
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	slices.Sort(titles)
	return titles
}

func lockFolder(t *testing.T, folder string, mode os.FileMode) {
	t.Helper()
	if err := os.Chmod(folder, mode); err != nil {
		t.Fatal(err)
	}
}

func captureScanLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	previous := slog.Default()
	var buffer bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &buffer
}

func assertRetainedWarning(t *testing.T, logs *bytes.Buffer, root string) {
	t.Helper()
	var record map[string]any
	for line := range strings.Lines(logs.String()) {
		if err := json.Unmarshal([]byte(line), &record); err == nil && record["operation"] == "library.scan" {
			break
		}
		record = nil
	}
	if record == nil || record["level"] != "WARN" || record["outcome"] != "retained" || record["failure_class"] != "permission" || record["retained"] != float64(1) {
		t.Fatalf("scan warning = %q", logs.String())
	}
	if strings.Contains(logs.String(), root) {
		t.Fatalf("scan warning exposed a Library path: %q", logs.String())
	}
	logs.Reset()
}

var allRetentionTitles = []string{"Deep", "Kept", "Visible"}

func TestFullScanKeepsTitlesOfAFolderThatIsTemporarilyUnreadable(t *testing.T) {
	root, folder := retentionLibrary(t)
	scanner := library.NewScanner(root, "Movies")
	if titles := sortedTitles(scanner.Scan(t.Context())); !slices.Equal(titles, allRetentionTitles) {
		t.Fatalf("first scan = %v", titles)
	}
	logs := captureScanLogs(t)
	lockFolder(t, folder, 0)
	if titles := sortedTitles(scanner.Scan(t.Context())); !slices.Equal(titles, allRetentionTitles) {
		t.Fatalf("scan with unreadable folder = %v", titles)
	}
	assertRetainedWarning(t, logs, root)
	lockFolder(t, folder, 0o700)
	if changed := scanner.Changed(t.Context()); changed[folder] == 0 {
		t.Fatalf("retained folder was not offered for retry: %v", changed)
	}
	if titles := sortedTitles(scanner.Scan(t.Context())); !slices.Equal(titles, allRetentionTitles) {
		t.Fatalf("scan after recovery = %v", titles)
	}
	if changed := scanner.Changed(t.Context()); len(changed) != 0 {
		t.Fatalf("recovered folder is still pending retry: %v", changed)
	}
}

func TestRescanKeepsTitlesOfATemporarilyUnreadableFolderAndRetriesIt(t *testing.T) {
	root, folder := retentionLibrary(t)
	scanner := library.NewScanner(root, "Movies")
	if titles := sortedTitles(scanner.Scan(t.Context())); !slices.Equal(titles, allRetentionTitles) {
		t.Fatalf("first scan = %v", titles)
	}
	logs := captureScanLogs(t)
	lockFolder(t, folder, 0)
	if titles := sortedTitles(scanner.Rescan(t.Context(), []string{folder})); !slices.Equal(titles, allRetentionTitles) {
		t.Fatalf("rescan with unreadable folder = %v", titles)
	}
	assertRetainedWarning(t, logs, root)
	if changed := scanner.Changed(t.Context()); changed[folder] == 0 {
		t.Fatalf("unreadable folder was not offered for retry: %v", changed)
	}
	lockFolder(t, folder, 0o700)
	writeScannerFile(t, filepath.Join(folder, "Added (2023).mp4"), "media")
	retry := slices.Collect(maps.Keys(scanner.Changed(t.Context())))
	if titles := sortedTitles(scanner.Rescan(t.Context(), retry)); !slices.Equal(titles, []string{"Added", "Deep", "Kept", "Visible"}) {
		t.Fatalf("retry after recovery = %v", titles)
	}
	if changed := scanner.Changed(t.Context()); len(changed) != 0 {
		t.Fatalf("recovered folder is still pending retry: %v", changed)
	}
}

func TestScanDropsTitlesOfAFolderThatDisappeared(t *testing.T) {
	root, folder := retentionLibrary(t)
	for name, scan := range map[string]func(*library.Scanner) ([]library.Item, error){
		"full": func(scanner *library.Scanner) ([]library.Item, error) { return scanner.Scan(t.Context()) },
		"incremental": func(scanner *library.Scanner) ([]library.Item, error) {
			return scanner.Rescan(t.Context(), []string{folder})
		},
	} {
		t.Run(name, func(t *testing.T) {
			writeScannerFile(t, filepath.Join(folder, "Kept (2021).mp4"), "media")
			writeScannerFile(t, filepath.Join(folder, "Nested", "Deep (2022).mp4"), "media")
			scanner := library.NewScanner(root, "Movies")
			if titles := sortedTitles(scanner.Scan(t.Context())); !slices.Equal(titles, allRetentionTitles) {
				t.Fatalf("first scan = %v", titles)
			}
			if err := os.RemoveAll(folder); err != nil {
				t.Fatal(err)
			}
			if titles := sortedTitles(scan(scanner)); !slices.Equal(titles, []string{"Visible"}) {
				t.Fatalf("scan after folder removal = %v", titles)
			}
		})
	}
}
