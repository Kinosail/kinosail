package downloads

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

const (
	maximumJobs     = 10000
	maximumJobBytes = 64 << 10
)

func (manager *Manager) load() error {
	jobs, err := manager.readJobs()
	if err != nil {
		return err
	}
	for _, job := range jobs {
		if job.State == "failed" {
			if err := manager.save(job); err != nil {
				return err
			}
		}
	}
	if manager.restoring == nil {
		manager.restoring = make(map[string]Job)
	}
	if manager.attempts == nil {
		manager.attempts = make(map[string]*attempt)
	}
	for id, job := range jobs {
		if job.State != "ready" {
			continue
		}
		manager.restoring[id] = job
		ctx, cancel := context.WithCancel(manager.ctx)
		manager.attempts[id] = &attempt{ctx, cancel}
		job.State, job.ReadyOffline, job.SHA256, job.Size = "preparing", false, "", 0
		jobs[id] = job
	}
	manager.jobs = jobs
	return nil
}

func (manager *Manager) readJobs() (map[string]Job, error) {
	entries, err := os.ReadDir(manager.root)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]Job{}, nil
	}
	if err != nil {
		return nil, err
	}
	return manager.readJobEntries(entries)
}

func (manager *Manager) readJobEntries(entries []os.DirEntry) (map[string]Job, error) {
	if tooManyJobEntries(entries) {
		return nil, errors.New("download state is invalid")
	}
	jobs := make(map[string]Job)
	for _, entry := range entries {
		job, included, err := manager.readJob(entry)
		if err != nil {
			return nil, err
		}
		if !included {
			continue
		}
		jobs[job.ID] = job
	}
	return jobs, nil
}

func tooManyJobEntries(entries []os.DirEntry) bool {
	count := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			count++
		}
	}
	return count > maximumJobs
}

func (manager *Manager) readJob(entry os.DirEntry) (Job, bool, error) {
	path := filepath.Join(manager.root, entry.Name())
	if strings.HasSuffix(entry.Name(), ".deleting") {
		err := os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
		return Job{}, false, err
	}
	if !strings.HasSuffix(entry.Name(), ".json") {
		return Job{}, false, nil
	}
	if !entry.Type().IsRegular() {
		return Job{}, false, errors.New("download state is invalid")
	}
	job, err := manager.loadJob(path)
	if err != nil || entry.Name() != job.ID+".json" {
		return Job{}, false, errors.New("download state is invalid")
	}
	return job, true, nil
}

func (manager *Manager) loadJob(path string) (Job, error) {
	file, err := os.Open(path) //nolint:gosec // Path is a direct child of the download cache.
	if err != nil {
		return Job{}, err
	}
	defer file.Close()
	var job Job
	if err := httpguard.DecodeJSON(file, maximumJobBytes, &job, false); err != nil || !validJob(job) {
		return Job{}, errors.New("download state is invalid")
	}
	job.File = filepath.Join(manager.root, job.ID+job.extension())
	if job.State == "preparing" {
		job.State, job.Error = "failed", "preparation was interrupted"
	}

	return job, nil
}

func validJob(job Job) bool {
	return validIdentity(job) && validFormat(job) && validOutcome(job) && validateSelection(job.Tracks) && len(job.SourceVersion) <= 128
}

func validIdentity(job Job) bool {
	return validID(job.ID) && job.ItemID != "" && len(job.ItemID) <= 256 && job.Profile != "" && len(job.Profile) <= 256 && job.Title != "" && len(job.Title) <= 512
}

func validFormat(job Job) bool {
	return oneOf(job.Quality, "original", "compatible", "1080p", "720p", "480p", "audio") && oneOf(job.State, "preparing", "ready", "failed") &&
		validExtension(job.Extension) && !job.Created.IsZero() && job.Size >= 0 && len(job.Error) <= 1000
}

func validID(id string) bool {
	if len(id) != 16 {
		return false
	}
	for _, character := range id {
		if !lowerHex(character) {
			return false
		}
	}
	return true
}

func validExtension(extension string) bool {
	if extension == "" {
		return true
	}
	if len(extension) < 2 || len(extension) > 16 || filepath.Ext(extension) != extension {
		return false
	}
	for _, character := range extension[1:] {
		if !alphaNumeric(character) {
			return false
		}
	}
	return true
}

func validOutcome(job Job) bool {
	digest := job.SHA256 == "" || validSHA256(job.SHA256)
	switch job.State {
	case "preparing":
		return validPreparing(job)
	case "ready":
		return validReady(job)
	case "failed":
		return validFailed(job, digest)
	default:
		return false
	}
}

func validSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !lowerHex(character) {
			return false
		}
	}
	return true
}

func lowerHex(character rune) bool {
	return character >= '0' && character <= '9' || character >= 'a' && character <= 'f'
}

func alphaNumeric(character rune) bool {
	return character >= '0' && character <= '9' || character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
}

func validPreparing(job Job) bool {
	return !job.ReadyOffline && job.Error == "" && job.SHA256 == "" && job.Size == 0
}

func validReady(job Job) bool {
	return job.ReadyOffline && job.Error == "" && validSHA256(job.SHA256)
}

func validFailed(job Job, digest bool) bool {
	return !job.ReadyOffline && job.Error != "" && digest
}

func (manager *Manager) save(job Job) error {
	if manager.root == "" {
		return nil
	}
	if !validJob(job) {
		return errors.New("download state is invalid")
	}
	if manager.persist == nil {
		return errors.New("download persistence is not configured")
	}
	return manager.persist(filepath.Join(manager.root, job.ID+".json"), job)
}

// Restore metadata first. One worker verifies payloads, preferring requested titles
// between files. Removed jobs cancel their current read before another block.
func (manager *Manager) verifyRestored() {
	for manager.ctx.Err() == nil {
		var id string
		select {
		case id = <-manager.requested:
		default:
		}
		job, work, exists := manager.restoredJob(id)
		if !exists {
			return
		}
		if work == nil {
			continue
		}
		manager.verifyRestoredJob(job, work)
	}
}

func (manager *Manager) restoredJob(id string) (Job, *attempt, bool) {
	manager.mu.RLock()
	job, exists := manager.restoring[id]
	if !exists {
		for candidate, value := range manager.restoring {
			id, job, exists = candidate, value, true
			break
		}
	}
	work := manager.attempts[id]
	manager.mu.RUnlock()
	return job, work, exists
}

func (manager *Manager) verifyRestoredJob(job Job, work *attempt) {
	manifest, err := sealManifestContext(work.ctx, job.File, job.ID)
	manager.mu.Lock()
	if manager.attempts[job.ID] != work {
		manager.mu.Unlock()
		return
	}
	if err == nil && !validManifest(manifest, job) {
		err = errors.New("stored download failed integrity verification")
	}
	if err == nil {
		err = manager.persist(job.File+".manifest", manifest)
	}
	if err != nil {
		job.State, job.ReadyOffline, job.Error = "failed", false, "stored download failed integrity verification"
	}
	if saveErr := manager.save(job); saveErr != nil {
		job.State, job.ReadyOffline, job.Error = "failed", false, "download state could not be saved"
	}
	manager.jobs[job.ID] = job
	delete(manager.restoring, job.ID)
	delete(manager.attempts, job.ID)
	manager.mu.Unlock()
	work.cancel()
	manager.emit(job)
}
