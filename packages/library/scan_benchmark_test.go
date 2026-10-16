package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func BenchmarkScan(b *testing.B) {
	b.StopTimer()
	root := b.TempDir()
	for index := range 512 {
		base := filepath.Join(root, fmt.Sprintf("Movie %04d", index))
		for name, data := range map[string]string{base + ".mp4": "media", base + ".nfo": `<movie><title>Movie</title><genre>Drama</genre></movie>`, base + ".jpg": "artwork", base + ".en.srt": "subtitle"} {
			if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	b.ReportAllocs()
	b.StartTimer()
	for b.Loop() {
		if items, err := ScanContext(b.Context(), root, "Movies"); err != nil || len(items) != 512 {
			b.Fatalf("Scan() = %d items, %v", len(items), err)
		}
	}
}

func incrementalTree(b *testing.B, files int) (string, string) {
	b.Helper()
	root := b.TempDir()
	changed := ""
	for index := range files / 4 {
		directory := filepath.Join(root, fmt.Sprintf("Show %03d", index/100), fmt.Sprintf("Season %02d", index%100/10))
		if err := os.MkdirAll(directory, 0o700); err != nil {
			b.Fatal(err)
		}
		base := filepath.Join(directory, fmt.Sprintf("Show S01E%03d", index%1000))
		for name, data := range map[string]string{base + ".mkv": "media", base + ".nfo": `<episodedetails><title>Episode</title></episodedetails>`, base + ".jpg": "artwork", base + ".en.srt": "subtitle"} {
			if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
				b.Fatal(err)
			}
		}
		changed = base + ".mkv"
	}
	return root, changed
}

func BenchmarkLibraryChange(b *testing.B) {
	root, changed := incrementalTree(b, 20_000)
	b.Run("full scan", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if items, err := ScanContext(b.Context(), root, "Shows"); err != nil || len(items) != 5_000 {
				b.Fatalf("Scan() = %d items, %v", len(items), err)
			}
		}
	})
	b.Run("incremental rescan", func(b *testing.B) {
		scanner := NewScanner(root, "Shows")
		if _, err := scanner.Scan(b.Context()); err != nil {
			b.Fatal(err)
		}
		b.ReportAllocs()
		for b.Loop() {
			if err := os.WriteFile(changed, []byte("changed media"), 0o600); err != nil {
				b.Fatal(err)
			}
			if items, err := scanner.Rescan(b.Context(), []string{changed}); err != nil || len(items) != 5_000 {
				b.Fatalf("Rescan() = %d items, %v", len(items), err)
			}
		}
	})
}

func BenchmarkOrganize(b *testing.B) {
	items := make([]Item, 10_000)
	for position := range items {
		items[position] = Item{ID: strconv.Itoa(position), Kind: "video", Show: fmt.Sprintf("Show %03d", position/100), Season: position/1_000 + 1, Episode: position%100 + 1}
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, shows := Organize(items); len(shows) != 100 {
			b.Fatal("shows were not organized")
		}
	}
}
