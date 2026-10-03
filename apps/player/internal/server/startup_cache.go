package server

import (
	"context"
	"os"
	"path/filepath"

	"github.com/MikeO7/kinosail/packages/playback"
)

// Read-only inspection never holds the job lock needed by real playback.
// Concurrent eviction can make inspection fail; preparation then yields.
func (startup *startupPreparation) cacheHeadroom(ctx context.Context) bool {
	size, err := playback.HLSCacheStats(startup.hls.cache, hlsPolicy())
	if err != nil || size+(64<<20) >= startup.limit || ctx.Err() != nil {
		return false
	}
	entries, err := os.ReadDir(startup.hls.cache)
	if os.IsNotExist(err) {
		return true
	}
	if err != nil || len(entries) > 4096 {
		return false
	}
	var speculative int64
	for _, entry := range entries {
		if ctx.Err() != nil {
			return false
		}
		if !entry.IsDir() || !playback.HLSCacheDirectory(entry.Name(), hlsPolicy()) {
			continue
		}
		directory := filepath.Join(startup.hls.cache, entry.Name())
		if _, err := os.Stat(filepath.Join(directory, ".startup")); err == nil {
			speculative += startupDirectoryBytes(directory)
		}
	}
	return speculative+(64<<20) < 256<<20
}
