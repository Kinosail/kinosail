package library

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type Scanner struct {
	root, namespace string
	mu              sync.Mutex
	dirs            map[string]*scanDir
	items           []Item
}

func NewScanner(root, namespace string) *Scanner {
	if root != "" {
		root = filepath.Clean(root)
	}
	return &Scanner{root: root, namespace: namespace}
}

func (scanner *Scanner) Root() string { return scanner.root }

func ScanContext(ctx context.Context, root, namespace string) ([]Item, error) {
	return NewScanner(root, namespace).Scan(ctx)
}

func (scanner *Scanner) Scan(ctx context.Context) ([]Item, error) {
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	return scanner.scan(ctx)
}

func (scanner *Scanner) scan(ctx context.Context) ([]Item, error) {
	if scanner.root == "" {
		return nil, nil
	}
	info, err := os.Lstat(scanner.root)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		scanner.dirs = nil
		return make([]Item, 0), ctx.Err()
	}
	if !info.IsDir() {
		scanner.dirs = nil
		return scanner.scanFile(ctx)
	}
	fresh, _, err := scanner.read(ctx, []string{scanner.root}, nil)
	if err != nil {
		return nil, err
	}
	scanner.compute(fresh, keySet(fresh))
	scanner.dirs = fresh
	return scanner.assemble(), nil
}

func (scanner *Scanner) Rescan(ctx context.Context, changed []string) ([]Item, error) {
	scanner.mu.Lock()
	defer scanner.mu.Unlock()
	if scanner.dirs == nil {
		return scanner.scan(ctx)
	}
	targets := scanner.targets(changed)
	if len(targets) == 0 {
		return scanner.items, ctx.Err()
	}
	fresh, missing, err := scanner.read(ctx, targets, scanner.dirs)
	if err != nil {
		return nil, err
	}
	for path := range missing {
		if parent := filepath.Dir(path); scanner.dirs[parent] != nil && fresh[parent] == nil {
			parents, _, parentErr := scanner.read(ctx, []string{parent}, scanner.dirs)
			if parentErr != nil {
				return nil, parentErr
			}
			maps.Copy(fresh, parents)
		}
	}
	next, affected := scanner.merge(fresh, missing)
	scanner.compute(next, affected)
	scanner.dirs = next
	return scanner.assemble(), nil
}

func (scanner *Scanner) merge(fresh map[string]*scanDir, missing map[string]bool) (map[string]*scanDir, map[string]bool) {
	next := maps.Clone(scanner.dirs)
	for path := range missing {
		deleteSubtree(next, path)
	}
	affected := keySet(fresh)
	for path, directory := range fresh {
		if previous := next[path]; previous != nil {
			for _, entry := range previous.entries {
				if entry.dir && !directory.hasDirectory(entry.name) {
					deleteSubtree(next, filepath.Join(path, entry.name))
				}
			}
			if filepath.Dir(path) == scanner.root {
				addSubtree(affected, next, path)
			}
		}
		next[path] = directory
	}
	return next, affected
}

func (scanner *Scanner) read(ctx context.Context, starts []string, known map[string]*scanDir) (map[string]*scanDir, map[string]bool, error) {
	var mu sync.Mutex
	fresh, missing := make(map[string]*scanDir), make(map[string]bool)
	failures := &scanFailures{}
	var rootErr error
	previous := scanner.dirs
	walkDirectories(ctx, starts, func(path string) []string {
		directory, err := readScanDir(path, previous[path], failures)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			switch {
			case path == scanner.root:
				rootErr = err
			case errors.Is(err, fs.ErrNotExist):
				missing[path] = true
			default:
				failures.record(err, true)
				if !retainSubtree(fresh, previous, path) {
					missing[path] = true
				} else {
					failures.retained.Add(1)
				}
			}
			return nil
		}
		fresh[path] = directory
		children := make([]string, 0)
		for _, entry := range directory.entries {
			child := filepath.Join(path, entry.name)
			if _, found := known[child]; entry.dir && !found {
				children = append(children, child)
			}
		}
		return children
	})
	failures.log()
	if rootErr != nil {
		return nil, nil, rootErr
	}
	return fresh, missing, ctx.Err()
}

func (failures *scanFailures) log() {
	directories, files := failures.directories.Load(), failures.files.Load()
	if directories == 0 && files == 0 {
		return
	}
	class, _ := failures.class.Load().(string)
	retained, outcome := failures.retained.Load(), "partial"
	if retained > 0 {
		outcome = "retained"
	}
	slog.Warn("Library scan skipped unreadable entries", "operation", "library.scan", "outcome", outcome, "failure_class", class, "directories", directories, "files", files, "retained", retained)
}

const retryModified = -2

func retainSubtree(fresh, previous map[string]*scanDir, path string) bool {
	known := previous[path]
	if known == nil {
		return false
	}
	prefix := path + string(filepath.Separator)
	for candidate, directory := range previous {
		if strings.HasPrefix(candidate, prefix) {
			fresh[candidate] = directory
		}
	}
	retained := *known
	retained.modified = retryModified
	fresh[path] = &retained
	return true
}

func (scanner *Scanner) targets(changed []string) []string {
	targets := make(map[string]bool)
	for _, path := range changed {
		path = filepath.Clean(path)
		relative, err := filepath.Rel(scanner.root, path)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		for scanner.dirs[path] == nil && path != scanner.root && filepath.Dir(path) != path {
			path = filepath.Dir(path)
		}
		targets[path] = true
	}
	return sortedKeys(targets)
}

func (directory *scanDir) hasDirectory(name string) bool {
	position, found := sort.Find(len(directory.entries), func(index int) int { return strings.Compare(name, directory.entries[index].name) })
	return found && directory.entries[position].dir
}

func deleteSubtree(dirs map[string]*scanDir, path string) {
	prefix := path + string(filepath.Separator)
	for candidate := range dirs {
		if candidate == path || strings.HasPrefix(candidate, prefix) {
			delete(dirs, candidate)
		}
	}
}

func addSubtree(affected map[string]bool, dirs map[string]*scanDir, path string) {
	prefix := path + string(filepath.Separator)
	for candidate := range dirs {
		if strings.HasPrefix(candidate, prefix) {
			affected[candidate] = true
		}
	}
}

func keySet[T any](values map[string]T) map[string]bool {
	keys := make(map[string]bool, len(values))
	for key := range values {
		keys[key] = true
	}
	return keys
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
