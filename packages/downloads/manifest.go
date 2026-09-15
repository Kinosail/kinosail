package downloads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// Verification blocks are independent of transfer extents. Clients may request
// several adjacent blocks together without losing block-sized recovery.
const (
	ChunkSize            int64 = 8 << 20
	MaximumChunks              = 16384
	maximumManifestBytes       = 2 << 20
)

type Manifest struct {
	Version   int      `json:"version"`
	ID        string   `json:"id"`
	Size      int64    `json:"size"`
	SHA256    string   `json:"sha256"`
	ChunkSize int64    `json:"chunkSize"`
	Chunks    []string `json:"chunks"`
}

func sealManifest(path, id string) (Manifest, error) {
	return sealManifestContext(context.Background(), path, id)
}

func sealManifestContext(ctx context.Context, path, id string) (Manifest, error) {
	file, err := os.Open(path) //nolint:gosec // Manager-owned cache path.
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	return digestFile(ctx, file, id, io.Discard)
}

type contextReader struct {
	ctx   context.Context
	input io.Reader
}

func (reader contextReader) Read(data []byte) (int, error) {
	if err := reader.ctx.Err(); err != nil {
		return 0, err
	}
	return reader.input.Read(data[:min(len(data), 128<<10)])
}

func digestFile(ctx context.Context, file *os.File, id string, destination io.Writer) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	info, err := file.Stat()
	if err != nil {
		return Manifest{}, err
	}
	if !validID(id) || !validDownloadSource(info) {
		return Manifest{}, errors.New("download size is unsupported")
	}
	result := Manifest{Version: 1, ID: id, Size: info.Size(), ChunkSize: ChunkSize, Chunks: []string{}}
	whole := sha256.New()
	for offset := int64(0); offset < result.Size; offset += ChunkSize {
		block := sha256.New()
		if _, err := io.CopyN(io.MultiWriter(destination, whole, block), contextReader{ctx, file}, min(ChunkSize, result.Size-offset)); err != nil {
			return Manifest{}, err
		}
		result.Chunks = append(result.Chunks, hex.EncodeToString(block.Sum(nil)))
	}
	after, err := file.Stat()
	if err != nil || !unchangedDownloadSource(info, after) {
		return Manifest{}, errors.New("download changed during verification")
	}
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	result.SHA256 = hex.EncodeToString(whole.Sum(nil))
	return result, nil
}

func copyManifest(ctx context.Context, input, output, id string) (Manifest, error) {
	if err := ctx.Err(); err != nil {
		return Manifest{}, err
	}
	source, err := os.Open(input) //nolint:gosec // Library-owned source.
	if err != nil {
		return Manifest{}, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil || !validDownloadSource(info) || !validID(id) {
		return Manifest{}, errors.New("download source is invalid")
	}
	destination, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // Manager-owned staging file.
	if err != nil {
		return Manifest{}, err
	}
	result, err := digestFile(ctx, source, id, destination)
	if err == nil {
		err = destination.Sync()
	}
	closeErr := destination.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(output)
		return Manifest{}, err
	}
	return result, nil
}

func validManifest(value Manifest, job Job) bool { //nolint:cyclop // Validate the complete sealed identity and every bounded digest before use.
	if value.Version != 1 || value.ID != job.ID || value.Size != job.Size || value.SHA256 != job.SHA256 ||
		value.ChunkSize != ChunkSize || value.Size <= 0 || value.Size > ChunkSize*MaximumChunks ||
		len(value.Chunks) != int((value.Size+ChunkSize-1)/ChunkSize) || !validSHA256(value.SHA256) {
		return false
	}
	for _, digest := range value.Chunks {
		if !validSHA256(digest) {
			return false
		}
	}
	return true
}

func readManifest(job Job) (Manifest, error) {
	file, err := os.Open(job.File + ".manifest") //nolint:gosec // Validated cache job.
	if err != nil {
		return Manifest{}, err
	}
	defer file.Close()
	var result Manifest
	if err := httpguard.DecodeJSON(file, maximumManifestBytes, &result, true); err != nil || !validManifest(result, job) {
		return Manifest{}, errors.New("download manifest is invalid")
	}
	return result, nil
}

type manifestFlight struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	result  Manifest
	err     error
}

// Manifest returns a sealed description only for the authorized, ready job.
func (manager *Manager) Manifest(profile, id string) (Manifest, error) {
	return manager.ManifestContext(context.Background(), profile, id)
}

// ManifestContext shares work per revision with bounded aggregate disk I/O.
func (manager *Manager) ManifestContext(ctx context.Context, profile, id string) (Manifest, error) {
	if !validID(id) {
		return Manifest{}, os.ErrNotExist
	}
	job, found := manager.Get(profile, id)
	if !found || job.State != "ready" {
		return Manifest{}, os.ErrNotExist
	}
	result, err := readManifest(job)
	if !errors.Is(err, os.ErrNotExist) {
		return result, err
	}
	flight, err := manager.manifestFlight(job) //nolint:contextcheck // Shared repair is owned by the manager and its last remaining waiter.
	if err != nil {
		return Manifest{}, err
	}
	defer func() {
		manager.manifestMu.Lock()
		flight.waiters--
		if flight.waiters == 0 {
			flight.cancel()
			if manager.manifestWork[id] == flight {
				delete(manager.manifestWork, id)
			}
		}
		manager.manifestMu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return Manifest{}, ctx.Err()
	case <-flight.done:
		return flight.result, flight.err
	}
}

func (manager *Manager) repairManifest(ctx context.Context, job Job, flight *manifestFlight) {
	defer close(flight.done)
	defer flight.cancel()
	select {
	case manager.manifestSlots <- struct{}{}:
		defer func() { <-manager.manifestSlots }()
	case <-ctx.Done():
		flight.err = ctx.Err()
		return
	}
	result, err := sealManifestContext(ctx, job.File, job.ID)
	if err != nil {
		flight.err = err
		return
	}
	if !validManifest(result, job) {
		flight.err = errors.New("stored download failed integrity verification")
		return
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	current, exists := manager.jobs[job.ID]
	if !exists || !current.Created.Equal(job.Created) || current.SHA256 != job.SHA256 || ctx.Err() != nil {
		flight.err = os.ErrNotExist
		return
	}
	flight.err = manager.persist(job.File+".manifest", result)
	flight.result = result
}

func (manager *Manager) manifestFlight(job Job) (*manifestFlight, error) {
	manager.manifestMu.Lock()
	if manager.manifestWork == nil {
		manager.manifestWork = make(map[string]*manifestFlight)
	}
	if manager.manifestSlots == nil {
		manager.manifestSlots = make(chan struct{}, 2)
	}
	flight := manager.manifestWork[job.ID]
	if flight == nil {
		if len(manager.manifestWork) >= QueueCapacity {
			manager.manifestMu.Unlock()
			return nil, ErrQueueFull
		}
		work, cancel := context.WithCancel(manager.ctx) //nolint:contextcheck // Shared repair outlives individual requests; the last waiter cancels it.
		flight = &manifestFlight{done: make(chan struct{}), cancel: cancel}
		manager.manifestWork[job.ID] = flight
		go manager.repairManifest(work, job, flight) //nolint:contextcheck // The last waiter cancels this shared repair, not the first request.
	}
	flight.waiters++
	manager.manifestMu.Unlock()
	return flight, nil
}

func validDownloadSource(info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Size() > 0 && info.Size() <= ChunkSize*MaximumChunks
}

func unchangedDownloadSource(before, after os.FileInfo) bool {
	return before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}
