package server

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func BenchmarkSubtitleMaintenanceCoveredLibrary(b *testing.B) {
	media, data := b.TempDir(), b.TempDir()
	provider := newSubtitleProvider(SubtitleConfig{URL: "http://127.0.0.1", APIKey: "key"}, b.TempDir(), data, nil, nil, "")
	manager := &subtitleManager{provider: provider}
	items := make([]library.Item, 500)
	for index := range items {
		id := fmt.Sprintf("%016x", index+1)
		video := filepath.Join(media, id+".mp4")
		sidecar := filepath.Join(media, id+".en.srt")
		contents := []byte("1\n00:00:01,000 --> 00:00:02,000\nReady\n")
		if err := os.WriteFile(video, []byte("video"), 0o600); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(sidecar, contents, 0o600); err != nil {
			b.Fatal(err)
		}
		items[index] = library.Item{ID: id, Kind: "video", Path: video, Subtitles: []string{sidecar}}
		record := completeSubtitleRecord(sidecar, contents, subtitleRecord{Source: "embedded", Score: 100, CheckedAt: time.Now().Unix(), Managed: true})
		if err := provider.ledger.store(subtitleRecordKey(id, "en"), record); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for range b.N {
		if result := manager.maintainItems(b.Context(), items, "en", 10, 0); result.Attempted != 0 {
			b.Fatalf("result = %#v", result)
		}
	}
}
