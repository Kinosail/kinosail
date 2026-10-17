package library

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

func (scanner *Scanner) Changed(ctx context.Context, within ...string) map[string]int64 {
	scanner.mu.Lock()
	known := make(map[string]int64, len(scanner.dirs))
	for path, directory := range scanner.dirs {
		if len(within) == 0 || slices.ContainsFunc(within, func(subtree string) bool { return pathWithin(path, subtree) }) {
			known[path] = directory.modified
		}
	}
	scanner.mu.Unlock()
	paths := make(chan string)
	changed := make(map[string]int64)
	var mu sync.Mutex
	var workers sync.WaitGroup
	for range min(scanIOWorkers(), max(1, len(known))) {
		workers.Go(func() {
			for path := range paths {
				current := int64(-1)
				if info, err := os.Lstat(path); err == nil && info.IsDir() {
					current = info.ModTime().UnixNano()
				}
				if current != known[path] {
					mu.Lock()
					changed[path] = current
					mu.Unlock()
				}
			}
		})
	}
	for path := range known {
		if ctx.Err() != nil {
			break
		}
		paths <- path
	}
	close(paths)
	workers.Wait()
	return changed
}

func pathWithin(path, subtree string) bool {
	return path == subtree || strings.HasPrefix(path, strings.TrimSuffix(subtree, string(filepath.Separator))+string(filepath.Separator))
}
