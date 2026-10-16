package library

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type scanDir struct {
	modified int64
	entries  []scanEntry
	regular  map[string]struct{}
	media    map[string]struct{}
	files    map[string]mediaFile
	items    map[string]Item
}

type scanEntry struct {
	name string
	dir  bool
	kind string
}

type mediaFile struct {
	size     int64
	modified time.Time
	sidecar  sidecarStamp
	metadata localMetadata
}

type sidecarStamp struct {
	present  bool
	size     int64
	modified int64
}

type scanFailures struct {
	directories atomic.Int64
	files       atomic.Int64
	class       atomic.Value
}

func (failures *scanFailures) record(err error, directory bool) {
	if directory {
		failures.directories.Add(1)
	} else {
		failures.files.Add(1)
	}
	class := "unreadable"
	if errors.Is(err, fs.ErrPermission) {
		class = "permission"
	}
	failures.class.CompareAndSwap(nil, class)
}

func scanIOWorkers() int {
	return min(16, max(4, 2*runtime.GOMAXPROCS(0)))
}

func readScanDir(path string, previous *scanDir, failures *scanFailures) (*scanDir, error) {
	directory, err := os.Open(path) // #nosec G304 -- directory beneath a configured Library root.
	if err != nil {
		return nil, err
	}
	defer func() { _ = directory.Close() }()
	info, err := directory.Stat()
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fs.ErrNotExist
	}
	listed, err := directory.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(listed, func(left, right fs.DirEntry) int { return strings.Compare(left.Name(), right.Name()) })
	result := &scanDir{modified: info.ModTime().UnixNano(), entries: make([]scanEntry, 0, len(listed)), regular: make(map[string]struct{}), media: make(map[string]struct{}), files: make(map[string]mediaFile)}
	for _, entry := range listed {
		if entry.IsDir() {
			result.entries = append(result.entries, scanEntry{name: entry.Name(), dir: true})
			continue
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			continue
		}
		if entry.Type().IsRegular() {
			result.regular[entry.Name()] = struct{}{}
		}
		kind := mediaKind(filepath.Ext(entry.Name()))
		if kind == "" {
			continue
		}
		result.entries = append(result.entries, scanEntry{name: entry.Name(), kind: kind})
		if kind != "photo" {
			result.media[strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))] = struct{}{}
		}
	}
	result.statMedia(path, listed, previous, failures)
	return result, nil
}

type statResult struct {
	file  mediaFile
	state int
}

func (directory *scanDir) statMedia(path string, listed []fs.DirEntry, previous *scanDir, failures *scanFailures) {
	byName := make(map[string]fs.DirEntry, len(listed))
	for _, entry := range listed {
		byName[entry.Name()] = entry
	}
	results := make([]statResult, len(directory.entries))
	stat := func(position int) {
		entry := directory.entries[position]
		if entry.dir {
			results[position].state = 1
			return
		}
		if entry.kind == "photo" && directory.sidecarArtwork(entry.name) {
			return
		}
		info, err := byName[entry.name].Info()
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				failures.record(err, false)
			}
			results[position].state = -1
			return
		}
		file := mediaFile{size: info.Size(), modified: info.ModTime()}
		file.sidecar, file.metadata = directory.sidecarMetadata(path, entry.name, byName, previous)
		results[position] = statResult{file: file, state: 2}
	}
	parallelEach(len(results), stat)
	kept := directory.entries[:0]
	for position, entry := range directory.entries {
		switch results[position].state {
		case -1:
			delete(directory.regular, entry.name)
		case 1:
			kept = append(kept, entry)
		case 2:
			directory.files[entry.name] = results[position].file
			kept = append(kept, entry)
		}
	}
	directory.entries = kept
}

func parallelEach(count int, work func(int)) {
	const serialLimit = 32
	if count <= serialLimit {
		for position := range count {
			work(position)
		}
		return
	}
	positions := make(chan int)
	var workers sync.WaitGroup
	for range min(scanIOWorkers(), count/serialLimit+1) {
		workers.Go(func() {
			for position := range positions {
				work(position)
			}
		})
	}
	for position := range count {
		positions <- position
	}
	close(positions)
	workers.Wait()
}

func (directory *scanDir) sidecarMetadata(path, name string, byName map[string]fs.DirEntry, previous *scanDir) (sidecarStamp, localMetadata) {
	sidecar := strings.TrimSuffix(name, filepath.Ext(name)) + ".nfo"
	if _, found := directory.regular[sidecar]; !found {
		return sidecarStamp{}, localMetadata{}
	}
	info, err := byName[sidecar].Info()
	if err != nil || !info.Mode().IsRegular() {
		return sidecarStamp{}, localMetadata{}
	}
	stamp := sidecarStamp{present: true, size: info.Size(), modified: info.ModTime().UnixNano()}
	if previous != nil {
		if cached, found := previous.files[name]; found && cached.sidecar == stamp {
			return stamp, cached.metadata
		}
	}
	return stamp, readMetadata(filepath.Join(path, sidecar))
}

func walkDirectories(ctx context.Context, starts []string, visit func(string) []string) {
	var mu sync.Mutex
	ready := sync.NewCond(&mu)
	queue, active := append([]string(nil), starts...), 0
	var workers sync.WaitGroup
	for range scanIOWorkers() {
		workers.Go(func() {
			for {
				mu.Lock()
				for len(queue) == 0 && active > 0 {
					ready.Wait()
				}
				if len(queue) == 0 {
					mu.Unlock()
					ready.Broadcast()
					return
				}
				path := queue[len(queue)-1]
				queue, active = queue[:len(queue)-1], active+1
				mu.Unlock()
				var children []string
				if ctx.Err() == nil {
					children = visit(path)
				}
				mu.Lock()
				queue, active = append(queue, children...), active-1
				mu.Unlock()
				ready.Broadcast()
			}
		})
	}
	workers.Wait()
}
