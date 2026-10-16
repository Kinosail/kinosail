package playback

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type HLSCacheControl struct {
	Cache  string
	Lock   sync.Locker
	Active func(string) bool
	Busy   func() bool
	Policy HLSRecipePolicy
	usage  *hlsCacheUsage
}

type hlsCacheUsage struct {
	mu   sync.Mutex
	call *hlsCacheMeasurement
}

type hlsCacheMeasurement struct {
	done chan struct{}
	size int64
	err  error
}

const hlsCacheDiscardPrefix = ".discard-"

func NewHLSCacheControl(cache string, lock sync.Locker, active func(string) bool, busy func() bool, policy HLSRecipePolicy) HLSCacheControl {
	return HLSCacheControl{Cache: cache, Lock: lock, Active: active, Busy: busy, Policy: policy, usage: &hlsCacheUsage{}}
}

func (control HLSCacheControl) Stats() (int64, error) {
	if control.usage == nil {
		return HLSCacheStats(control.Cache, control.Policy)
	}
	usage := control.usage
	usage.mu.Lock()
	call := usage.call
	if call == nil {
		call = &hlsCacheMeasurement{done: make(chan struct{})}
		usage.call = call
		usage.mu.Unlock()
		call.size, call.err = HLSCacheStats(control.Cache, control.Policy)
		usage.mu.Lock()
		usage.call = nil
		close(call.done)
	}
	usage.mu.Unlock()
	<-call.done
	return call.size, call.err
}

func (control HLSCacheControl) Clear() error {
	control.Lock.Lock()
	if control.Busy() {
		control.Lock.Unlock()
		return errors.New("transcodes are currently active")
	}
	discarded, err := detachHLSCache(control.Cache, control.Policy)
	control.Lock.Unlock()
	return errors.Join(err, removeDiscardedHLS(discarded))
}

func (control HLSCacheControl) Prune(limit int64) (int64, error) {
	return pruneHLSCache(control.Cache, limit, control.Active, control.Policy, control.Lock)
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
		bytes, err := hlsCacheDirectoryBytes(filepath.Join(cache, entry.Name()))
		if err != nil {
			return 0, err
		}
		size += bytes
	}
	return size, nil
}

func hlsCacheDirectoryBytes(directory string) (int64, error) {
	var size int64
	err := filepath.WalkDir(directory, func(_ string, entry fs.DirEntry, walkErr error) error {
		if errors.Is(walkErr, fs.ErrNotExist) {
			return nil
		}
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		info, err := entry.Info()
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func detachHLSCache(cache string, policy HLSRecipePolicy) ([]string, error) {
	if cache == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(cache)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	discarded := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), hlsCacheDiscardPrefix) {
			discarded = append(discarded, filepath.Join(cache, entry.Name()))
			continue
		}
		if !HLSCacheDirectory(entry.Name(), policy) {
			continue
		}
		trash, err := detachHLSCacheEntry(cache, entry.Name())
		if err != nil {
			return discarded, err
		}
		if trash != "" {
			discarded = append(discarded, trash)
		}
	}
	return discarded, nil
}

func detachHLSCacheEntry(cache, name string) (string, error) {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return "", err
	}
	trash := filepath.Join(cache, hlsCacheDiscardPrefix+name+"-"+hex.EncodeToString(suffix[:]))
	err := os.Rename(filepath.Join(cache, name), trash)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return trash, nil
}

func removeDiscardedHLS(discarded []string) error {
	var result error
	for _, path := range discarded {
		result = errors.Join(result, os.RemoveAll(path))
	}
	return result
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
