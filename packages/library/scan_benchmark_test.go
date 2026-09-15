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
