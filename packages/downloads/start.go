package downloads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
)

func (manager *Manager) Start(profileID string, item library.Item, quality string) (Job, error) {
	return manager.StartSelected(profileID, item, quality, nil)
}

func (manager *Manager) StartSelected(profileID string, item library.Item, quality string, selection *TrackSelection) (Job, error) {
	if manager.root == "" {
		return Job{}, errors.New("offline download cache is not configured")
	}
	if !validateSelection(selection) || quality == "original" && selection != nil {
		return Job{}, errors.New("download track selection is invalid")
	}
	job, err := manager.prepareJob(profileID, item, quality, selection)
	if err != nil {
		return Job{}, err
	}
	job, work, err := manager.reserveJob(job)
	if err != nil || work == nil {
		return job, err
	}
	manager.emit(job)
	return manager.enqueue(job, item, work)
}

func (manager *Manager) prepareJob(profileID string, item library.Item, quality string, selection *TrackSelection) (Job, error) {
	job, err := newJob(manager.root, profileID, item, quality)
	if err != nil {
		return Job{}, err
	}
	job.Tracks, job.SourceVersion = selection, playback.SourceVersion(item.Path)
	recipe, _ := json.Marshal(selection)
	revision := sha256.Sum256([]byte(job.ID + "\x00" + job.SourceVersion + "\x00tracks-v1\x00" + string(recipe)))
	job.ID = hex.EncodeToString(revision[:8])
	manager.mu.RLock()
	_, known := manager.jobs[job.ID]
	full := !known && len(manager.jobs) >= maximumJobs
	manager.mu.RUnlock()
	if full {
		return Job{}, ErrCapacity
	}
	if quality != "original" && item.Kind == "video" {
		if manager.inspect == nil {
			return Job{}, errors.New("download media inspection is unavailable")
		}
		_, _, matroska, err := resolveTracks(manager.inspect(manager.ctx, item), item, selection)
		if err != nil {
			return Job{}, err
		}
		if matroska {
			job.Extension = ".mkv"
		}
	}
	job.File = filepath.Join(manager.root, job.ID+job.extension())
	return job, nil
}

func (manager *Manager) reserveJob(job Job) (Job, *attempt, error) {
	manager.mu.Lock()
	if manager.err != nil {
		manager.mu.Unlock()
		return Job{}, nil, manager.err
	}
	if existing, ok := manager.jobs[job.ID]; ok {
		if existing.Profile != job.Profile || existing.ItemID != job.ItemID || existing.Quality != job.Quality || existing.SourceVersion != job.SourceVersion {
			manager.mu.Unlock()
			return Job{}, nil, errors.New("download revision conflict")
		}
		if existing.State != "failed" {
			manager.mu.Unlock()
			return existing, nil, nil
		}
	}
	if _, known := manager.jobs[job.ID]; !known && len(manager.jobs) >= maximumJobs {
		manager.mu.Unlock()
		return Job{}, nil, ErrCapacity
	}
	if err := manager.ctx.Err(); err != nil {
		manager.mu.Unlock()
		return Job{}, nil, err
	}
	select {
	case manager.pending <- struct{}{}:
	default:
		manager.mu.Unlock()
		return Job{}, nil, ErrQueueFull
	}
	if err := os.MkdirAll(manager.root, 0o700); err != nil {
		manager.mu.Unlock()
		<-manager.pending
		return Job{}, nil, err
	}
	if err := manager.save(job); err != nil {
		manager.mu.Unlock()
		<-manager.pending
		return Job{}, nil, err
	}
	ctx, cancel := context.WithCancel(manager.ctx)
	work := &attempt{ctx, cancel}
	if manager.attempts == nil {
		manager.attempts = make(map[string]*attempt)
	}
	manager.attempts[job.ID] = work
	manager.jobs[job.ID] = job
	manager.mu.Unlock()
	return job, work, nil
}

func (manager *Manager) enqueue(job Job, item library.Item, work *attempt) (Job, error) {
	select {
	case manager.tasks <- task{job: job, item: item, attempt: work}:
		return job, nil
	case <-manager.ctx.Done():
		manager.mu.Lock()
		work.cancel()
		if manager.attempts[job.ID] == work {
			delete(manager.jobs, job.ID)
			delete(manager.attempts, job.ID)
			_ = os.Remove(filepath.Join(manager.root, job.ID+".json"))
		}
		manager.mu.Unlock()
		<-manager.pending
		return Job{}, manager.ctx.Err()
	}
}

func newJob(root, profileID string, item library.Item, quality string) (Job, error) {
	if !oneOf(quality, "original", "compatible", "1080p", "720p", "480p", "audio") || !oneOf(item.Kind, "video", "audio", "audiobook") || (quality == "audio" && item.Kind == "video" || quality == "compatible" && item.Kind != "video") {
		return Job{}, errors.New("download quality is invalid for this item")
	}
	sum := sha256.Sum256([]byte(profileID + "\x00" + item.ID + "\x00" + quality + "\x00" + item.Added.UTC().String()))
	id := hex.EncodeToString(sum[:8])
	extension := Job{Quality: quality}.extension()
	if quality == "original" {
		extension = filepath.Ext(item.Path)
	}
	job := Job{ID: id, ItemID: item.ID, Profile: profileID, Title: item.Title, Quality: quality, State: "preparing", Extension: extension, File: filepath.Join(root, id+extension), Created: time.Now().UTC()}
	if !validJob(job) {
		return Job{}, errors.New("download state is invalid")
	}
	return job, nil
}
