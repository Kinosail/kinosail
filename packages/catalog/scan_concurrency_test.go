package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestScanRootsPreservesOrderAndFailureBoundary(t *testing.T) { //nolint:cyclop // One fixture checks ordered success, failure, cancellation, and empty roots.
	first := t.TempDir()
	last := t.TempDir()
	for directory, title := range map[string]string{first: "Zulu", last: "Alpha"} {
		if err := os.WriteFile(filepath.Join(directory, title+".mp4"), []byte("media"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	roots := []ScanRoot{{first, "First"}, {last, "Last"}}
	decorations := 0
	decorate := func(items []library.Item) []library.Item { decorations++; return items }
	items, err := Scan(t.Context(), roots, decorate)
	if err != nil || len(items) != 2 || items[0].Title != "Zulu" || items[1].Title != "Alpha" || decorations != 1 {
		t.Fatalf("scan = %#v, %v, decorations = %d", items, err, decorations)
	}

	missing := filepath.Join(t.TempDir(), "missing")
	items, err = Scan(t.Context(), []ScanRoot{{first, "First"}, {missing, "Missing"}, {last, "Last"}}, decorate)
	if err == nil || len(items) != 1 || items[0].Title != "Zulu" || decorations != 1 {
		t.Fatalf("failed scan = %#v, %v, decorations = %d", items, err, decorations)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	items, err = Scan(ctx, roots, decorate)
	if !errors.Is(err, context.Canceled) || len(items) != 0 || decorations != 1 {
		t.Fatalf("canceled scan = %#v, %v, decorations = %d", items, err, decorations)
	}
	items, err = Scan(ctx, nil, decorate)
	if err != nil || len(items) != 0 || decorations != 2 {
		t.Fatalf("empty scan = %#v, %v, decorations = %d", items, err, decorations)
	}
}

func BenchmarkScanRoots(b *testing.B) { //nolint:cyclop,gocognit // The benchmark builds representative roots and compares complete scans.
	b.StopTimer()
	base := b.TempDir()
	roots := make([]ScanRoot, 4)
	for root := range roots {
		path := filepath.Join(base, fmt.Sprintf("Library-%d", root))
		if err := os.Mkdir(path, 0o700); err != nil {
			b.Fatal(err)
		}
		roots[root] = ScanRoot{Path: path, Namespace: fmt.Sprintf("Movies-%d", root)}
		for index := range 256 {
			stem := filepath.Join(path, fmt.Sprintf("Movie-%04d", index))
			for suffix, data := range map[string]string{".mp4": "media", ".nfo": `<movie><title>Movie</title><genre>Drama</genre></movie>`, ".jpg": "artwork", ".en.srt": "subtitle"} {
				if err := os.WriteFile(stem+suffix, []byte(data), 0o600); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	for _, mode := range []string{"serial", "parallel"} {
		b.Run(mode, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				var items []library.Item
				var err error
				if mode == "serial" {
					for _, root := range roots {
						var scanned []library.Item
						scanned, err = library.ScanContext(b.Context(), root.Path, root.Namespace)
						items = append(items, scanned...)
						if err != nil {
							break
						}
					}
				} else {
					items, err = Scan(b.Context(), roots, nil)
				}
				if err != nil || len(items) != 1024 {
					b.Fatalf("scan = %d items, %v", len(items), err)
				}
			}
		})
	}
}
