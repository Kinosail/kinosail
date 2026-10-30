package downloads

import (
	"errors"
	"os"
	"sort"
	"time"
)

const recentDownloadUse = 24 * time.Hour

type Trim struct {
	Bytes      int64
	FreedBytes int64
	Removed    int
}

type trimCandidate struct {
	job   Job
	bytes int64
	used  time.Time
}

func (manager *Manager) Trim(limit int64, now time.Time) (Trim, error) {
	var result Trim
	if manager.root == "" {
		return result, nil
	}
	manager.mu.RLock()
	candidates := make([]trimCandidate, 0, len(manager.jobs))
	for _, job := range manager.jobs {
		info, err := os.Stat(job.File)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		result.Bytes += info.Size()
		if job.State == "ready" {
			candidates = append(candidates, trimCandidate{job, info.Size(), lastUse(job)})
		}
	}
	manager.mu.RUnlock()
	sort.Slice(candidates, func(left, right int) bool { return candidates[left].used.Before(candidates[right].used) })
	var trimErr error
	for _, candidate := range candidates {
		if result.Bytes <= limit || now.Sub(candidate.used) < recentDownloadUse {
			break
		}
		if err := manager.remove(candidate.job.Profile, candidate.job.ID, deliveryFiles{remove: os.Remove, rename: os.Rename}); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				trimErr = errors.Join(trimErr, err)
			}
			continue
		}
		result.Removed++
		result.Bytes -= candidate.bytes
		result.FreedBytes += candidate.bytes
	}
	return result, trimErr
}

func lastUse(job Job) time.Time {
	used := job.Created
	if info, err := os.Stat(useMarker(job)); err == nil && info.ModTime().After(used) {
		used = info.ModTime()
	}
	return used
}

func useMarker(job Job) string { return job.File + ".used" }

func markUse(job Job, now time.Time) {
	marker := useMarker(job)
	info, err := os.Stat(marker)
	if err == nil && now.Sub(info.ModTime()) < time.Hour {
		return
	}
	if errors.Is(err, os.ErrNotExist) {
		file, createErr := os.OpenFile(marker, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if createErr != nil {
			return
		}
		_ = file.Close()
	}
	_ = os.Chtimes(marker, now, now)
}
