package playback

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type hlsCacheEntry struct {
	name     string
	size     int64
	modified time.Time
}

func pruneHLSCache(cache string, limit int64, active func(string) bool, policy HLSRecipePolicy, lock sync.Locker) (int64, error) {
	if cache == "" {
		return 0, nil
	}
	if limit < 0 || active == nil {
		return 0, errors.New("HLS cache prune input is invalid")
	}
	candidates, total, err := pruneCandidates(cache, policy)
	if err != nil {
		return total, err
	}
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].modified.Before(candidates[right].modified) })
	return evictHLSCacheEntries(candidates, total, limit, func(name string) (string, bool, error) {
		lock.Lock()
		defer lock.Unlock()
		if active(name) {
			return "", false, nil
		}
		trash, err := detachHLSCacheEntry(cache, name)
		return trash, true, err
	})
}

func evictHLSCacheEntries(candidates []hlsCacheEntry, total, limit int64, detach func(string) (string, bool, error)) (int64, error) {
	for _, candidate := range candidates {
		if total <= limit {
			break
		}
		trash, evicted, err := detach(candidate.name)
		if err != nil {
			return total, err
		}
		if !evicted {
			continue
		}
		if err := removeDiscardedHLS([]string{trash}); trash != "" && err != nil {
			return total, err
		}
		total -= candidate.size
	}
	return total, nil
}

func pruneCandidates(cache string, policy HLSRecipePolicy) ([]hlsCacheEntry, int64, error) {
	entries, err := os.ReadDir(cache)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	discarded := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), hlsCacheDiscardPrefix) {
			discarded = append(discarded, filepath.Join(cache, entry.Name()))
		}
	}
	if err := removeDiscardedHLS(discarded); err != nil {
		return nil, 0, err
	}
	return prunableHLSEntries(cache, entries, policy)
}

func prunableHLSEntries(cache string, entries []os.DirEntry, policy HLSRecipePolicy) ([]hlsCacheEntry, int64, error) {
	candidates, total := make([]hlsCacheEntry, 0), int64(0)
	for _, entry := range entries {
		if !entry.IsDir() || !HLSCacheDirectory(entry.Name(), policy) {
			continue
		}
		size, modified, err := hlsCacheEntryInfo(filepath.Join(cache, entry.Name()), entry)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, total, err
		}
		total += size
		candidates = append(candidates, hlsCacheEntry{entry.Name(), size, modified})
	}
	return candidates, total, nil
}

func hlsCacheEntryInfo(path string, entry os.DirEntry) (int64, time.Time, error) {
	size, err := hlsCacheDirectoryBytes(path)
	if err != nil {
		return 0, time.Time{}, err
	}
	info, err := entry.Info()
	if err != nil {
		return 0, time.Time{}, err
	}
	return size, info.ModTime(), nil
}
