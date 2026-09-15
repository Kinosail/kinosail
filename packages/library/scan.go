package library

import (
	"context"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type scanFile struct {
	path  string
	entry fs.DirEntry
	kind  string
}

type scanResult struct {
	item     Item
	included bool
	err      error
}

type scanCatalog struct {
	files         map[string]struct{}
	media         map[string]struct{}
	subtitleFiles []string
	subtitles     map[string][]string
}

// ScanContext returns the playable files below root and stops when ctx is canceled.
func ScanContext(ctx context.Context, root, namespace string) ([]Item, error) {
	return scanContextWith(ctx, root, namespace, scanFiles)
}

func scanContextWith(ctx context.Context, root, namespace string, scan func(context.Context, string, string, *scanCatalog, []scanFile) ([]scanResult, error)) ([]Item, error) {
	if root == "" {
		return nil, nil
	}
	catalog, files, err := catalogScan(ctx, root)
	if err != nil {
		return nil, err
	}
	catalog.indexSubtitles()
	results, err := scan(ctx, root, namespace, catalog, files)
	if err != nil {
		return nil, err
	}
	return scannedItems(results)
}

func catalogScan(ctx context.Context, root string) (*scanCatalog, []scanFile, error) {
	catalog := &scanCatalog{files: make(map[string]struct{}), media: make(map[string]struct{}), subtitles: make(map[string][]string)}
	files := make([]scanFile, 0)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || entry.IsDir() || entry.Type()&fs.ModeSymlink != 0 {
			return err
		}
		if entry.Type().IsRegular() {
			catalog.add(path)
		}
		if kind := mediaKind(filepath.Ext(path)); kind != "" {
			files = append(files, scanFile{path, entry, kind})
			if kind != "photo" {
				catalog.media[strings.TrimSuffix(path, filepath.Ext(path))] = struct{}{}
			}
		}
		return nil
	})
	return catalog, files, err
}

func scanFiles(ctx context.Context, root, namespace string, catalog *scanCatalog, files []scanFile) ([]scanResult, error) {
	results := make([]scanResult, len(files))
	jobs := make(chan int)
	var wait sync.WaitGroup
	wait.Add(scanWorkers(len(files)))
	for range scanWorkers(len(files)) {
		go scanWorker(ctx, root, namespace, catalog, files, results, jobs, &wait)
	}
	for position := range files {
		if ctx.Err() != nil {
			close(jobs)
			wait.Wait()
			return nil, ctx.Err()
		}
		jobs <- position
	}
	close(jobs)
	wait.Wait()
	return results, ctx.Err()
}

func scanWorkers(files int) int {
	return min(max(1, runtime.GOMAXPROCS(0)-1), files)
}

func scanWorker(ctx context.Context, root, namespace string, catalog *scanCatalog, files []scanFile, results []scanResult, jobs <-chan int, wait *sync.WaitGroup) {
	defer wait.Done()
	for position := range jobs {
		if ctx.Err() != nil {
			continue
		}
		file := files[position]
		if file.kind == "photo" && catalog.sidecarArtwork(file.path) {
			continue
		}
		item, err := scanItem(root, namespace, file.path, file.entry, file.kind, catalog)
		results[position] = scanResult{item, true, err}
	}
}

func scannedItems(results []scanResult) ([]Item, error) {
	items := make([]Item, 0, len(results))
	for _, result := range results {
		if result.err != nil {
			return nil, result.err
		}
		if result.included {
			items = append(items, result.item)
		}
	}
	sort.Slice(items, func(left, right int) bool { return items[left].Title < items[right].Title })
	return items, nil
}

func (catalog *scanCatalog) add(path string) {
	catalog.files[path] = struct{}{}
	if extension := strings.ToLower(filepath.Ext(path)); extension == ".vtt" || extension == ".srt" {
		catalog.subtitleFiles = append(catalog.subtitleFiles, path)
	}
}

func (catalog *scanCatalog) first(paths ...string) string {
	for _, path := range paths {
		if _, found := catalog.files[path]; found {
			return path
		}
	}
	return ""
}

func (catalog *scanCatalog) indexSubtitles() {
	for _, path := range catalog.subtitleFiles {
		stem := strings.TrimSuffix(path, filepath.Ext(path))
		for base := stem; ; {
			if catalog.hasMedia(base) {
				catalog.subtitles[base] = append(catalog.subtitles[base], path)
				break
			}
			next := strings.TrimSuffix(base, filepath.Ext(base))
			if next == base {
				break
			}
			base = next
		}
	}
	for base := range catalog.subtitles {
		sort.Strings(catalog.subtitles[base])
	}
}

func (catalog *scanCatalog) hasMedia(base string) bool {
	_, found := catalog.media[base]
	return found
}
