package playback

import (
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// HLSCacheControl serializes application cache inspection and mutation.
type HLSCacheControl struct {
	Cache  string
	Lock   sync.Locker
	Active func(string) bool
	Busy   func() bool
	Policy HLSRecipePolicy
}

// NewHLSCacheControl binds one application's cache state to shared operations.
func NewHLSCacheControl(cache string, lock sync.Locker, active func(string) bool, busy func() bool, policy HLSRecipePolicy) HLSCacheControl {
	return HLSCacheControl{Cache: cache, Lock: lock, Active: active, Busy: busy, Policy: policy}
}

// Stats returns the current cache size while holding the application lock.
func (control HLSCacheControl) Stats() (int64, error) {
	control.Lock.Lock()
	defer control.Lock.Unlock()
	return HLSCacheStats(control.Cache, control.Policy)
}

// Clear removes inactive cache entries when no transcodes are active.
func (control HLSCacheControl) Clear() error {
	control.Lock.Lock()
	defer control.Lock.Unlock()
	if control.Busy() {
		return errors.New("transcodes are currently active")
	}
	return ClearHLSCache(control.Cache, control.Policy)
}

// Prune evicts old inactive entries while holding the application lock.
func (control HLSCacheControl) Prune(limit int64) (int64, error) {
	control.Lock.Lock()
	defer control.Lock.Unlock()
	return PruneHLSCache(control.Cache, limit, control.Active, control.Policy)
}

func HLSCacheStats(cache string, policy HLSRecipePolicy) (int64, error) {
	if cache == "" {
		return 0, nil
	}
	entries, err := os.ReadDir(cache)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var size int64
	for _, entry := range entries {
		if !entry.IsDir() || !HLSCacheDirectory(entry.Name(), policy) {
			continue
		}
		err = filepath.Walk(filepath.Join(cache, entry.Name()), func(_ string, info os.FileInfo, walkErr error) error {
			if walkErr == nil && !info.IsDir() {
				size += info.Size()
			}
			return walkErr
		})
		if err != nil {
			return 0, err
		}
	}
	return size, nil
}

func ClearHLSCache(cache string, policy HLSRecipePolicy) error {
	if cache == "" {
		return nil
	}
	entries, err := os.ReadDir(cache)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() && HLSCacheDirectory(entry.Name(), policy) {
			if err := os.RemoveAll(filepath.Join(cache, entry.Name())); err != nil { //nolint:gosec // Validated cache key is below the configured root.
				return err
			}
		}
	}
	return nil
}

type hlsCacheEntry struct {
	name     string
	size     int64
	modified time.Time
}

func PruneHLSCache(cache string, limit int64, active func(string) bool, policy HLSRecipePolicy) (int64, error) { //nolint:cyclop,gocognit // Accounting and oldest-first eviction are one operation.
	if cache == "" {
		return 0, nil
	}
	if limit < 0 || active == nil {
		return 0, errors.New("HLS cache prune input is invalid")
	}
	candidates, total, err := pruneCandidates(cache, active, policy)
	if err != nil {
		return total, err
	}
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].modified.Before(candidates[right].modified) })
	return evictHLSCacheEntries(cache, candidates, total, limit)
}

func evictHLSCacheEntries(cache string, candidates []hlsCacheEntry, total, limit int64) (int64, error) {
	for _, candidate := range candidates {
		if total <= limit {
			break
		}
		if err := os.RemoveAll(filepath.Join(cache, candidate.name)); err != nil { //nolint:gosec // Validated cache key is below the configured root.
			return total, err
		}
		total -= candidate.size
	}
	return total, nil
}

func pruneCandidates(cache string, active func(string) bool, policy HLSRecipePolicy) ([]hlsCacheEntry, int64, error) {
	entries, err := os.ReadDir(cache)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	return prunableHLSEntries(cache, entries, active, policy)
}

func prunableHLSEntries(cache string, entries []os.DirEntry, active func(string) bool, policy HLSRecipePolicy) ([]hlsCacheEntry, int64, error) {
	candidates, total := make([]hlsCacheEntry, 0), int64(0)
	for _, entry := range entries {
		if !entry.IsDir() || !HLSCacheDirectory(entry.Name(), policy) {
			continue
		}
		size, modified, err := hlsCacheEntryInfo(filepath.Join(cache, entry.Name()), entry)
		if err != nil {
			return nil, total, err
		}
		total += size
		if !active(entry.Name()) {
			candidates = append(candidates, hlsCacheEntry{entry.Name(), size, modified})
		}
	}
	return candidates, total, nil
}

func hlsCacheEntryInfo(path string, entry os.DirEntry) (int64, time.Time, error) {
	size := int64(0)
	if err := filepath.Walk(path, func(_ string, info os.FileInfo, walkErr error) error {
		if walkErr == nil && !info.IsDir() {
			size += info.Size()
		}
		return walkErr
	}); err != nil {
		return 0, time.Time{}, err
	}
	info, err := entry.Info()
	if err != nil {
		return 0, time.Time{}, err
	}
	return size, info.ModTime(), nil
}

func HLSCacheDirectory(name string, policy HLSRecipePolicy) bool {
	if id, token, planned := strings.Cut(name, "-plan-"); planned {
		if !validMediaID(id) {
			return false
		}
		_, err := ParseHLSRecipe(token, policy)
		return err == nil
	}
	id, track, audio := strings.Cut(name, "-audio-")
	if !validMediaID(id) {
		return false
	}
	if !audio {
		return true
	}
	value, err := strconv.Atoi(track)
	return err == nil && value > 0 && value <= 31
}

func validMediaID(id string) bool {
	if len(id) != 16 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
