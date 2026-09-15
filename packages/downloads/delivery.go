package downloads

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Get returns one job only when the profile owns it.
func (manager *Manager) Get(profileID, id string) (Job, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	job, found := manager.jobs[id]
	if found && job.Profile == profileID && manager.restoring[id].ID != "" {
		select {
		case manager.requested <- id:
		default:
		}
	}
	return job, found && job.Profile == profileID
}

// List returns newest-first jobs owned by one profile.
func (manager *Manager) List(profileID string) []Job {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	jobs := make([]Job, 0)
	for _, job := range manager.jobs {
		if job.Profile == profileID {
			jobs = append(jobs, job)
		}
	}
	sort.Slice(jobs, func(left, right int) bool { return jobs[left].Created.After(jobs[right].Created) })
	return jobs
}

// Remove stages media and state deletion before it changes memory.
func (manager *Manager) Remove(profileID, id string) error {
	return manager.remove(profileID, id, deliveryFiles{remove: os.Remove, rename: os.Rename})
}

type deliveryFiles struct {
	remove func(string) error
	rename func(string, string) error
	open   func(string) (*os.File, error)
}

func (manager *Manager) remove(profileID, id string, files deliveryFiles) error {
	manager.mu.Lock()
	job, found := manager.jobs[id]
	if !found || job.Profile != profileID {
		manager.mu.Unlock()
		return os.ErrNotExist
	}
	mediaTrash, mediaMoved, err := stageRemoval(job.File, files)
	if err != nil {
		manager.mu.Unlock()
		return err
	}
	state := filepath.Join(manager.root, job.ID+".json")
	stateTrash, _, err := stageRemoval(state, files)
	if err != nil {
		if mediaMoved {
			err = errors.Join(err, files.rename(mediaTrash, job.File))
		}
		manager.mu.Unlock()
		return err
	}
	if work := manager.attempts[id]; work != nil {
		work.cancel()
		delete(manager.attempts, id)
	}
	manager.manifestMu.Lock()
	if flight := manager.manifestWork[id]; flight != nil {
		flight.cancel()
		delete(manager.manifestWork, id)
	}
	manager.manifestMu.Unlock()
	delete(manager.restoring, id)
	delete(manager.jobs, id)
	// Remove the old sidecar before another attempt can reuse this identity.
	_ = os.Remove(job.File + ".manifest")
	manager.mu.Unlock()
	_ = os.Remove(mediaTrash)
	_ = os.Remove(stateTrash)
	manager.emit(job)
	return nil
}

func stageRemoval(path string, files deliveryFiles) (string, bool, error) {
	trash := path + ".deleting"
	if err := files.remove(trash); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", false, err
	}
	if err := files.rename(path, trash); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	return trash, true, nil
}

// Wait waits until a profile-owned job finishes or its context stops.
func (manager *Manager) Wait(ctx context.Context, profileID, id string) (Job, error) {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		job, found := manager.Get(profileID, id)
		if !found {
			return Job{}, os.ErrNotExist
		}
		switch job.State {
		case "ready":
			return job, nil
		case "failed":
			return Job{}, errors.New(job.Error)
		}
		select {
		case <-ctx.Done():
			return Job{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Serve writes one validated job with immutable integrity headers.
func Serve(writer http.ResponseWriter, request *http.Request, job Job) error {
	return serve(writer, request, job, deliveryFiles{open: os.Open})
}

func serve(writer http.ResponseWriter, request *http.Request, job Job, files deliveryFiles) error {
	file, err := files.open(job.File) //nolint:gosec // A validated job owns this path.
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != job.Size {
		return errors.New("prepared download size changed")
	}
	writer.Header().Set("ETag", `"`+job.SHA256+`"`)
	if digest, err := hex.DecodeString(job.SHA256); err == nil {
		writer.Header().Set("Repr-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(digest)+":")
	}
	setRangeDigest(writer.Header(), request.Header.Get("Range"), info.Size(), job)
	writer.Header().Set("Cache-Control", "private, no-transform")
	writer.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, job.Title+filepath.Ext(job.File)))
	if !conditionalRequest(request) && completedRange(request.Header.Get("Range"), info.Size()) {
		writer.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(info.Size(), 10))
		writer.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		return nil
	}
	http.ServeContent(rangeDigestWriter{writer}, request, filepath.Base(job.File), info.ModTime(), file)
	return nil
}

// ServeContent decides whether conditional requests actually deliver a range.
// A sealed block digest is only valid for that partial response.
type rangeDigestWriter struct{ http.ResponseWriter }

func (writer rangeDigestWriter) WriteHeader(status int) {
	if status != http.StatusPartialContent {
		writer.Header().Del("Content-Digest")
	}
	writer.ResponseWriter.WriteHeader(status)
}

func conditionalRequest(request *http.Request) bool {
	for _, header := range []string{"If-Range", "If-Match", "If-None-Match", "If-Unmodified-Since", "If-Modified-Since"} {
		if request.Header.Get(header) != "" {
			return true
		}
	}
	return false
}

func completedRange(value string, size int64) bool {
	start, end, found := strings.Cut(strings.TrimPrefix(strings.TrimSpace(value), "bytes="), "-")
	position, err := strconv.ParseInt(start, 10, 64)
	return found && end == "" && err == nil && position == size
}

func explicitRange(value string, size int64) (int64, int64, bool) {
	if len(value) > 128 || size < 1 || !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false
	}
	startText, endText, found := strings.Cut(strings.TrimPrefix(value, "bytes="), "-")
	start, startErr := strconv.ParseInt(startText, 10, 64)
	end, endErr := strconv.ParseInt(endText, 10, 64)
	if !found || startErr != nil || endErr != nil || start < 0 || start >= size || end < start {
		return 0, 0, false
	}
	end = min(end, size-1)
	return start, end - start + 1, true
}

// Configured reports whether the manager has a persistent cache root.
func (manager *Manager) Configured() bool { return manager.root != "" }

// Err returns a startup state error.
func (manager *Manager) Err() error {
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.err
}

// SetPublisher replaces the event callback.
func (manager *Manager) SetPublisher(publish func(Job)) {
	manager.mu.Lock()
	manager.publish = publish
	manager.mu.Unlock()
}

func (manager *Manager) emit(job Job) {
	manager.mu.RLock()
	publish := manager.publish
	manager.mu.RUnlock()
	if publish != nil {
		publish(job)
	}
}

func setRangeDigest(headers http.Header, requested string, size int64, job Job) {
	// Integrity belongs to the sealed manifest. Hashing arbitrary ranges here
	// rereads gigabytes before the first response byte and also delays HEAD.
	if start, length, ranged := explicitRange(requested, size); ranged && start%ChunkSize == 0 && length == min(ChunkSize, size-start) {
		if manifest, err := readManifest(job); err == nil && start/ChunkSize < int64(len(manifest.Chunks)) {
			digest, _ := hex.DecodeString(manifest.Chunks[start/ChunkSize])
			headers.Set("Content-Digest", "sha-256=:"+base64.StdEncoding.EncodeToString(digest)+":")
		}
	}
}
