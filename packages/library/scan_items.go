package library

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

type scanView struct {
	dirs      map[string]*scanDir
	subtitles map[string][]string
}

func (scanner *Scanner) compute(dirs map[string]*scanDir, affected map[string]bool) {
	paths := make(chan string)
	var workers sync.WaitGroup
	for range max(1, runtime.GOMAXPROCS(0)) {
		workers.Go(func() {
			for path := range paths {
				if directory := dirs[path]; directory != nil {
					directory.items = scanner.directoryItems(dirs, path, directory)
				}
			}
		})
	}
	for path := range affected {
		paths <- path
	}
	close(paths)
	workers.Wait()
}

func (scanner *Scanner) directoryItems(dirs map[string]*scanDir, path string, directory *scanDir) map[string]Item {
	view := scanView{dirs: dirs, subtitles: directory.subtitleIndex(path)}
	built := make([]Item, len(directory.entries))
	included := make([]bool, len(directory.entries))
	parallelEach(len(directory.entries), func(position int) {
		entry := directory.entries[position]
		if file, found := directory.files[entry.name]; !entry.dir && found {
			built[position], included[position] = view.item(scanner.root, scanner.namespace, filepath.Join(path, entry.name), entry.kind, file), true
		}
	})
	items := make(map[string]Item, len(directory.files))
	for position, entry := range directory.entries {
		if included[position] {
			items[entry.name] = built[position]
		}
	}
	return items
}

func (scanner *Scanner) assemble() []Item {
	items := make([]Item, 0)
	var visit func(string)
	visit = func(path string) {
		directory := scanner.dirs[path]
		if directory == nil {
			return
		}
		for _, entry := range directory.entries {
			if entry.dir {
				visit(filepath.Join(path, entry.name))
			} else if item, found := directory.items[entry.name]; found {
				items = append(items, item)
			}
		}
	}
	visit(scanner.root)
	sort.SliceStable(items, func(left, right int) bool { return items[left].Title < items[right].Title })
	scanner.items = items
	return items
}

func (scanner *Scanner) scanFile(ctx context.Context) ([]Item, error) {
	parent, name := filepath.Dir(scanner.root), filepath.Base(scanner.root)
	directory := &scanDir{regular: make(map[string]struct{}), media: make(map[string]struct{}), files: make(map[string]mediaFile)}
	info, err := os.Lstat(scanner.root)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		directory.regular[name] = struct{}{}
	}
	kind := mediaKind(filepath.Ext(name))
	if kind != "photo" && kind != "" {
		directory.media[strings.TrimSuffix(name, filepath.Ext(name))] = struct{}{}
	}
	items := make([]Item, 0, 1)
	if kind != "" && (kind != "photo" || !directory.sidecarArtwork(name)) {
		view := scanView{dirs: map[string]*scanDir{parent: directory}}
		item := view.item(scanner.root, scanner.namespace, scanner.root, kind, mediaFile{size: info.Size(), modified: info.ModTime()})
		item.Path = scanner.root
		items = append(items, item)
	}
	return items, ctx.Err()
}

func (view scanView) first(paths ...string) string {
	for _, path := range paths {
		if directory := view.dirs[filepath.Dir(path)]; directory != nil {
			if _, found := directory.regular[filepath.Base(path)]; found {
				return path
			}
		}
	}
	return ""
}

func (view scanView) firstArtwork(bases ...string) string {
	for _, base := range bases {
		if path := view.first(base+".jpg", base+".jpeg", base+".png", base+".webp"); path != "" {
			return path
		}
	}
	return ""
}

func (view scanView) sidecarSubtitles(base string) []string {
	paths := make([]string, 0, len(view.subtitles[base]))
	if exact := view.first(base+".vtt", base+".srt"); exact != "" {
		paths = append(paths, exact)
	}
	for _, path := range view.subtitles[base] {
		if path != base+".vtt" && path != base+".srt" {
			paths = append(paths, path)
		}
	}
	return paths
}

func (directory *scanDir) subtitleIndex(path string) map[string][]string {
	subtitles := make(map[string][]string)
	for name := range directory.regular {
		if extension := strings.ToLower(filepath.Ext(name)); extension != ".vtt" && extension != ".srt" {
			continue
		}
		for stem := strings.TrimSuffix(name, filepath.Ext(name)); ; {
			if _, found := directory.media[stem]; found {
				base := filepath.Join(path, stem)
				subtitles[base] = append(subtitles[base], filepath.Join(path, name))
				break
			}
			next := strings.TrimSuffix(stem, filepath.Ext(stem))
			if next == stem {
				break
			}
			stem = next
		}
	}
	for base := range subtitles {
		sort.Strings(subtitles[base])
	}
	return subtitles
}

func (directory *scanDir) sidecarArtwork(name string) bool {
	if _, found := directory.media[strings.TrimSuffix(name, filepath.Ext(name))]; found {
		return true
	}
	lower := strings.ToLower(strings.TrimSuffix(name, filepath.Ext(name)))
	switch lower {
	case "poster", "fanart", "backdrop", "thumb", "folder", "clearlogo", "logo":
		return true
	}
	for _, suffix := range []string{"-thumb", "-fanart", "-backdrop", "-clearlogo", "-logo"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
