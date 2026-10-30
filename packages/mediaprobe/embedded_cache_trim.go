package mediaprobe

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	embeddedRecentUse     = 24 * time.Hour
	abandonedOCRWorkspace = 24 * time.Hour
)

type EmbeddedTrim struct {
	Bytes      int64
	FreedBytes int64
	Groups     int
	Workspaces int
}

type cacheGroup struct {
	path  string
	bytes int64
	used  time.Time
}

func TrimEmbeddedCache(cacheDir string, limit int64, now time.Time) (EmbeddedTrim, error) {
	var result EmbeddedTrim
	if cacheDir == "" {
		return result, nil
	}
	root := filepath.Join(cacheDir, "embedded-subtitles")
	groups, workspaces, err := scanEmbeddedCache(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return EmbeddedTrim{}, nil
		}
		return result, err
	}
	var removeErr error
	for _, workspace := range workspaces {
		if now.Sub(workspace.used) < abandonedOCRWorkspace {
			continue
		}
		if err := os.RemoveAll(workspace.path); err != nil {
			removeErr = errors.Join(removeErr, err)
			continue
		}
		result.Workspaces++
		result.FreedBytes += workspace.bytes
	}
	sort.Slice(groups, func(left, right int) bool { return groups[left].used.Before(groups[right].used) })
	for _, group := range groups {
		result.Bytes += group.bytes
	}
	for _, group := range groups {
		if result.Bytes <= limit || now.Sub(group.used) < embeddedRecentUse {
			break
		}
		if err := os.RemoveAll(group.path); err != nil {
			removeErr = errors.Join(removeErr, err)
			continue
		}
		_ = os.Remove(filepath.Dir(group.path))
		result.Groups++
		result.Bytes -= group.bytes
		result.FreedBytes += group.bytes
	}
	return result, removeErr
}

func scanEmbeddedCache(root string) ([]cacheGroup, []cacheGroup, error) {
	groups, workspaces := map[string]*cacheGroup{}, map[string]*cacheGroup{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator))), "/")
		if len(parts) < 3 {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		key := filepath.Join(root, parts[0], parts[1])
		group := record(groups, key, 0, info.ModTime())
		if strings.HasPrefix(parts[2], ".ocr-") {
			record(workspaces, filepath.Join(key, parts[2]), info.Size(), info.ModTime())
		} else {
			group.bytes += info.Size()
		}
		return nil
	})
	return cacheGroups(groups), cacheGroups(workspaces), err
}

func record(groups map[string]*cacheGroup, key string, bytes int64, used time.Time) *cacheGroup {
	group := groups[key]
	if group == nil {
		group = &cacheGroup{path: key}
		groups[key] = group
	}
	group.bytes += bytes
	if used.After(group.used) {
		group.used = used
	}
	return group
}

func cacheGroups(groups map[string]*cacheGroup) []cacheGroup {
	result := make([]cacheGroup, 0, len(groups))
	for _, group := range groups {
		result = append(result, *group)
	}
	return result
}

func markEmbeddedUse(path string, now time.Time) {
	info, err := os.Stat(path)
	if err == nil && now.Sub(info.ModTime()) > time.Hour {
		_ = os.Chtimes(path, now, now)
	}
}
