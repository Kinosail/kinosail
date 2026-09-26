package markers

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
)

func TestMarkerBatchBoundsConcurrentGroupsAndPersistsSuccess(t *testing.T) { //nolint:cyclop // The barrier proves bounded overlap and durable publication.
	if runtime.GOMAXPROCS(0) < 3 {
		t.Skip("one worker is expected with fewer than three process CPUs")
	}
	items := make([]library.Item, 4)
	for index := range items {
		items[index] = library.Item{ID: fmt.Sprintf("movie-%d", index), Kind: "video", Library: fmt.Sprintf("Library-%d", index), Size: 1, Added: time.Unix(1, 0)}
	}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir()})
	analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 100} })
	entered := make(chan struct{}, len(items))
	release := make(chan struct{})
	var active, maximum atomic.Int32
	analyzer.extractCredits = func(ctx context.Context, _ library.Item, _, _ float64) ([]byte, error) {
		current := active.Add(1)
		for previous := maximum.Load(); current > previous; previous = maximum.Load() {
			if maximum.CompareAndSwap(previous, current) {
				break
			}
		}
		defer active.Add(-1)
		entered <- struct{}{}
		select {
		case <-release:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	done := make(chan error, 1)
	go func() { done <- analyzer.analyzeBatch(t.Context(), items) }()
	for range 2 {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("independent marker groups did not overlap")
		}
	}
	close(release)
	if err := <-done; err != nil || maximum.Load() != 2 || len(analyzer.records) != len(items) {
		t.Fatalf("batch = %d records, maximum %d workers, %v", len(analyzer.records), maximum.Load(), err)
	}
	if reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)}); len(reloaded.records) != len(items) {
		t.Fatalf("persisted records = %d", len(reloaded.records))
	}
}

func TestMarkerBatchFailureStillPublishesOtherGroups(t *testing.T) {
	items := []library.Item{
		{ID: "good", Kind: "video", Library: "Good", Size: 1, Added: time.Unix(1, 0)},
		{ID: "bad", Kind: "video", Library: "Bad", Size: 1, Added: time.Unix(1, 0)},
	}
	analyzer := NewAnalyzer(Config{DataDir: t.TempDir()})
	analyzer.SetProbe(func(_ context.Context, item library.Item) Media {
		if item.ID == "bad" {
			return Media{}
		}
		return Media{Duration: 100}
	})
	analyzer.extractCredits = func(context.Context, library.Item, float64, float64) ([]byte, error) { return nil, nil }
	if err := analyzer.analyzeBatch(t.Context(), items); err == nil || len(analyzer.records) != 1 {
		t.Fatalf("batch = %d records, %v", len(analyzer.records), err)
	}
	if _, found := analyzer.records["good"]; !found {
		t.Fatal("successful group was not published")
	}
	if reloaded := NewAnalyzer(Config{DataDir: filepathDir(analyzer.file)}); len(reloaded.records) != 1 {
		t.Fatalf("persisted records = %d", len(reloaded.records))
	}
}

func BenchmarkMarkerBatch(b *testing.B) { //nolint:cyclop,gocognit // Real FFmpeg extraction compares the complete serial and concurrent batches.
	b.StopTimer()
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		b.Skip("ffmpeg is unavailable")
	}
	path := filepath.Join(b.TempDir(), "movie.mp4")
	command := exec.CommandContext(b.Context(), ffmpeg, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "color=c=black:s=160x90:r=1", "-t", "100", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", path)
	if output, err := command.CombinedOutput(); err != nil {
		b.Fatalf("make marker fixture: %v: %s", err, output)
	}
	items := make([]library.Item, 4)
	for index := range items {
		items[index] = library.Item{ID: fmt.Sprintf("movie-%d", index), Kind: "video", Library: fmt.Sprintf("Library-%d", index), Path: path, Size: 1, Added: time.Unix(1, 0)}
	}
	for _, mode := range []string{"serial", "parallel"} {
		b.Run(mode, func(b *testing.B) {
			analyzer := NewAnalyzer(Config{DataDir: b.TempDir(), FFmpeg: ffmpeg})
			analyzer.SetProbe(func(context.Context, library.Item) Media { return Media{Duration: 100} })
			b.ReportAllocs()
			for b.Loop() {
				var err error
				if mode == "serial" {
					for _, item := range items {
						if err = analyzer.analyzeGroup(b.Context(), []library.Item{item}); err != nil {
							break
						}
					}
				} else {
					err = analyzer.analyzeBatch(b.Context(), items)
				}
				if err != nil || len(analyzer.records) != len(items) {
					b.Fatalf("marker batch = %d records, %v", len(analyzer.records), err)
				}
			}
		})
	}
}
