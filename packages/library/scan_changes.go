package library

import (
	"context"
	"os"
	"sync"
)

func (scanner *Scanner) Changed(ctx context.Context) map[string]int64 {
	scanner.mu.Lock()
	known := make(map[string]int64, len(scanner.dirs))
	for path, directory := range scanner.dirs {
		known[path] = directory.modified
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
