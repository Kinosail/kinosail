package server

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkHLSSegmentKeepAliveDuringCacheStats(b *testing.B) {
	cache := b.TempDir()
	for directory := range 64 {
		root := filepath.Join(cache, fmt.Sprintf("%016x", directory), "360p")
		if err := os.MkdirAll(root, 0o700); err != nil {
			b.Fatal(err)
		}
		for segment := range 300 {
			if err := os.WriteFile(filepath.Join(root, fmt.Sprintf("segment-%05d.m4s", segment)), []byte("x"), 0o600); err != nil {
				b.Fatal(err)
			}
		}
	}
	manager := &hlsManager{ctx: b.Context(), cache: cache, jobs: make(map[string]*hlsJob)}
	manager.configureCache()
	key := "00000000000000ff"
	manager.jobs[key] = &hlsJob{activity: make(chan struct{}, 1)}
	ctx, stop := context.WithCancel(b.Context())
	measured := make(chan struct{})
	go func() {
		defer close(measured)
		for ctx.Err() == nil {
			_, _ = manager.cacheOps.Stats()
		}
	}()
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			manager.keepHLSAlive(key, ctx)
		}
	})
	b.StopTimer()
	stop()
	<-measured
}
