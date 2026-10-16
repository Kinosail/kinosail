package catalog

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func benchmarkTree(b *testing.B, episodes int, extension string) string {
	b.Helper()
	root := b.TempDir()
	for index := range episodes {
		directory := filepath.Join(root, fmt.Sprintf("Show %03d", index/100), fmt.Sprintf("Season %02d", index%100/10))
		if err := os.MkdirAll(directory, 0o700); err != nil {
			b.Fatal(err)
		}
		base := filepath.Join(directory, fmt.Sprintf("Show S01E%03d", index%1000))
		for _, name := range []string{base + extension, base + ".nfo", base + ".jpg", base + ".en.srt"} {
			if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	return root
}

func BenchmarkStartupFirstPublish(b *testing.B) {
	root := benchmarkTree(b, 200, ".flac")
	slow := func(ctx context.Context, items []library.Item) []library.Item {
		for position := range items {
			if items[position].Kind == "audio" {
				time.Sleep(2 * time.Millisecond)
			}
		}
		return items
	}
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithCancel(b.Context())
		index := NewDecoratedIndex(ctx, []ScanRoot{{Path: root, Namespace: "Music"}}, "", time.Hour, nil, Decoration{Enrich: slow, Enriches: func(item library.Item) bool { return item.Kind == "audio" }, Background: true})
		if snapshot, err := index.Library(); err != nil || snapshot.Len() != 200 {
			b.Fatalf("first publish = %d, %v", snapshot.Len(), err)
		}
		cancel()
	}
}

func BenchmarkPollCycle(b *testing.B) {
	root := benchmarkTree(b, 5_000, ".mkv")
	roots := []ScanRoot{{Path: root, Namespace: "Shows"}}
	index := NewIndex(b.Context(), roots, "", time.Hour, nil)
	b.ReportAllocs()
	for b.Loop() {
		if changed := index.changedDirectories(b.Context(), roots); len(changed) != 0 {
			b.Fatalf("unchanged tree reported %d directories", len(changed))
		}
	}
}
