package library

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanRecognizesExtendedMediaFiles(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	extensions := []string{"av1", "divx", "dvr-ms", "h264", "h265", "h266", "hevc", "mxf", "ogm", "rm", "rmvb", "vvc", "wtv", "ac3", "dts", "eac3", "tta", "wma", "aifc", "alac", "ape", "caf", "mpc", "wv"}
	for _, extension := range extensions {
		if err := os.WriteFile(filepath.Join(directory, "Example."+strings.ToUpper(extension)), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, extension := range []string{"strm", "strmlnk", "exe", "unknown"} {
		if err := os.WriteFile(filepath.Join(directory, "Excluded."+extension), []byte("https://example.invalid/private"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	items, err := ScanContext(t.Context(), directory, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != len(extensions) {
		t.Fatalf("items=%d want=%d", len(items), len(extensions))
	}
	for _, item := range items {
		if strings.HasPrefix(item.Title, "Excluded") {
			t.Fatalf("unsupported input indexed: %s", item.Title)
		}
	}
}
