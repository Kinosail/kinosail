package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/catalog"
)

func TestSubtitleEditReleasesSidecarLockBeforeLibraryRefresh(t *testing.T) {
	t.Parallel()
	manager, item, request, data := subtitleReviewFixture(t)
	var blocking atomic.Bool
	entered, release := make(chan struct{}, 1), make(chan struct{})
	acquire := func(context.Context) (func(), error) {
		if blocking.CompareAndSwap(true, false) {
			entered <- struct{}{}
			<-release
		}
		return func() {}, nil
	}
	index := &libraryIndex{Index: catalog.NewDecoratedIndex(t.Context(), []catalog.ScanRoot{{Path: filepath.Dir(item.Path)}}, t.TempDir(), 0, acquire, catalog.Decoration{})}
	manager.index, manager.provider.index = index, index
	items, err := index.Snapshot()
	if err != nil || len(items) != 1 {
		t.Fatalf("scanned items = %d, %v", len(items), err)
	}
	blocking.Store(true)
	statuses := make(chan int, 1)
	go func() {
		_, status, _ := manager.applySubtitleEdit(request, items[0].ID, subtitleEdit{Language: "en", Fingerprint: subtitleFingerprint(data), Offset: 500})
		statuses <- status
	}()
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("saved edit never refreshed the Library")
	}
	unlocked := manager.provider.sidecar.TryLock()
	if unlocked {
		manager.provider.sidecar.Unlock()
	}
	close(release)
	if status := <-statuses; !unlocked || status != 200 {
		t.Fatalf("edit held the sidecar lock during the Library refresh: unlocked=%v status=%d", unlocked, status)
	}
}

func BenchmarkSidecarWriteRefresh(b *testing.B) {
	media := b.TempDir()
	for show := range 500 {
		directory := filepath.Join(media, fmt.Sprintf("Show %03d", show))
		if err := os.MkdirAll(directory, 0o700); err != nil {
			b.Fatal(err)
		}
		for episode := range 10 {
			base := filepath.Join(directory, fmt.Sprintf("Show %03d S01E%02d", show, episode+1))
			for _, name := range []string{base + ".mkv", base + ".en.srt", base + ".es.srt", base + ".nfo"} {
				if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
					b.Fatal(err)
				}
			}
		}
	}
	index := &libraryIndex{Index: catalog.NewDecoratedIndex(b.Context(), []catalog.ScanRoot{{Path: media}}, b.TempDir(), 0, nil, catalog.Decoration{})}
	if items, _, _ := index.Status(); items != 5000 {
		b.Fatalf("library has %d items, want 5000", items)
	}
	manager := &subtitleManager{index: index}
	episode := filepath.Join(media, "Show 250", "Show 250 S01E05.mkv")
	sidecar := filepath.Join(media, "Show 250", "Show 250 S01E05.fr.srt")
	b.ReportAllocs()
	b.ResetTimer()
	for position := range b.N {
		if position%2 == 0 {
			if err := os.WriteFile(sidecar, []byte("x"), 0o600); err != nil {
				b.Fatal(err)
			}
		} else if err := os.Remove(sidecar); err != nil {
			b.Fatal(err)
		}
		if err := manager.refreshSidecarFolders(b.Context(), episode); err != nil {
			b.Fatal(err)
		}
	}
}
