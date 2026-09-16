package downloads

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/MikeO7/kinosail/packages/library"
	"github.com/MikeO7/kinosail/packages/playback"
	"github.com/MikeO7/kinosail/packages/transcodepolicy"
)

// QueueCapacity bounds pending work while playback keeps priority.
const QueueCapacity = 32

var (
	ErrCapacity  = errors.New("remove a prepared download before adding another")
	ErrQueueFull = errors.New("offline download queue is full")
)

type attempt struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// Job describes one private offline download.
type Job struct {
	SourceVersion string          `json:"sourceVersion,omitempty"`
	Tracks        *TrackSelection `json:"tracks,omitempty"`
	ID            string          `json:"id"`
	ItemID        string          `json:"itemId"`
	Profile       string          `json:"profileId,omitempty"`
	Title         string          `json:"title"`
	Quality       string          `json:"quality"`
	State         string          `json:"state"`
	SHA256        string          `json:"sha256,omitempty"`
	Size          int64           `json:"size,omitempty"`
	ReadyOffline  bool            `json:"readyOffline"`
	Error         string          `json:"error,omitempty"`
	File          string          `json:"-"`
	Extension     string          `json:"extension,omitempty"`
	Created       time.Time       `json:"created"`
}

func (job Job) extension() string {
	if job.Extension != "" {
		return job.Extension
	}
	if job.Quality == "audio" {
		return ".m4a"
	}
	return ".mp4"
}

type task struct {
	job     Job
	item    library.Item
	attempt *attempt
}

// Config injects application policy into the shared download lifecycle.
type Config struct {
	ServerID        string
	Context         context.Context
	Cache           string
	FFmpeg          string
	Inspect         func(context.Context, library.Item) playback.MediaFacts
	AcquireEncoding func(context.Context, transcodepolicy.Settings) (func(), error)
	Transcoding     func(software bool) transcodepolicy.Settings
	Acquire         func(context.Context) (func(), error)
	Persist         func(string, any) error
	Publish         func(Job)
}

// Manager owns durable jobs, bounded work, integrity, and delivery.
type Manager struct {
	serverID        string
	manifestMu      sync.Mutex
	manifestWork    map[string]*manifestFlight
	manifestSlots   chan struct{}
	attempts        map[string]*attempt
	restoring       map[string]Job
	requested       chan string
	ctx             context.Context
	root            string
	ffmpeg          string
	inspect         func(context.Context, library.Item) playback.MediaFacts
	acquireEncoding func(context.Context, transcodepolicy.Settings) (func(), error)
	transcoding     func(bool) transcodepolicy.Settings
	acquire         func(context.Context) (func(), error)
	persist         func(string, any) error
	publish         func(Job)
	mu              sync.RWMutex
	jobs            map[string]Job
	pending         chan struct{}
	tasks           chan task
	err             error
}

// New loads durable state and starts one bounded worker.
func New(config Config) *Manager { //nolint:cyclop // Startup wires bounded workers and recovery state together.
	ctx := config.Context
	if ctx == nil {
		ctx = context.Background()
	}
	manager := &Manager{
		serverID: config.ServerID, ctx: ctx, ffmpeg: config.FFmpeg, inspect: config.Inspect, acquireEncoding: config.AcquireEncoding, transcoding: config.Transcoding,
		acquire: config.Acquire, persist: config.Persist, publish: config.Publish,
		jobs: make(map[string]Job), attempts: make(map[string]*attempt), restoring: make(map[string]Job), requested: make(chan string, QueueCapacity), manifestWork: make(map[string]*manifestFlight), manifestSlots: make(chan struct{}, 2), pending: make(chan struct{}, QueueCapacity+1), tasks: make(chan task, QueueCapacity),
	}
	if config.Cache == "" {
		return manager
	}
	manager.root = filepath.Join(config.Cache, "downloads")
	if err := manager.load(); err != nil {
		manager.err = err
		return manager
	}
	go manager.work()
	go manager.verifyRestored()
	return manager
}

// Start validates, persists, and queues an idempotent job.
func (manager *Manager) work() {
	for {
		select {
		case <-manager.ctx.Done():
			return
		case next := <-manager.tasks:
			if next.attempt.ctx.Err() == nil {
				manager.prepareTask(next)
			}
			next.attempt.cancel()
			manager.mu.Lock()
			if manager.attempts[next.job.ID] == next.attempt {
				delete(manager.attempts, next.job.ID)
			}
			manager.mu.Unlock()
			<-manager.pending
		}
	}
}

func (manager *Manager) prepareTask(next task) {
	job, item := next.job, next.item
	ctx := manager.ctx
	if next.attempt != nil {
		ctx = next.attempt.ctx
	}
	if ctx.Err() != nil {
		return
	}
	temporary := job.File + ".part" + job.extension()
	_ = os.Remove(temporary)
	manifest, err := manager.prepareFile(ctx, item, job, temporary)
	manager.mu.Lock()
	current, exists := manager.jobs[job.ID]
	if !exists || current.State != "preparing" || !current.Created.Equal(job.Created) || next.attempt != nil && manager.attempts[job.ID] != next.attempt {
		manager.mu.Unlock()
		_ = os.Remove(temporary)
		return
	}
	job = manager.publishPrepared(ctx, job, temporary, manifest, err)
	manager.jobs[job.ID] = job
	manager.mu.Unlock()
	manager.emit(job)
}

func (manager *Manager) reserveContext(ctx context.Context) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if manager.acquire == nil {
		return func() {}, nil
	}
	return manager.acquire(ctx)
}

func (manager *Manager) settings(software bool) transcodepolicy.Settings {
	if manager.transcoding == nil {
		return transcodepolicy.Settings{}
	}
	return manager.transcoding(software)
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func (manager *Manager) prepareFile(ctx context.Context, item library.Item, job Job, temporary string) (Manifest, error) {
	var manifest Manifest
	var err error
	if job.Quality == "original" {
		var release func()
		release, err = manager.reserveContext(ctx)
		if err == nil {
			manifest, err = copyManifest(ctx, item.Path, temporary, job.ID)
			release()
		}
	} else {
		err = manager.encodeSelectedContext(ctx, item, job.Quality, temporary, false, job.Tracks)
	}
	primary := manager.settings(false)
	var commandError *exec.ExitError
	if errors.As(err, &commandError) && ctx.Err() == nil && job.Quality != "original" && primary.Accelerator != "none" {
		_ = os.Remove(temporary)
		err = manager.encodeSelectedContext(ctx, item, job.Quality, temporary, true, job.Tracks)
	}
	if err == nil && job.Quality != "original" {
		manifest, err = sealManifestContext(ctx, temporary, job.ID)
	}
	return manifest, err
}

// publishPrepared runs with manager.mu held after checking attempt ownership.
func (manager *Manager) publishPrepared(ctx context.Context, job Job, temporary string, manifest Manifest, err error) Job {
	if err == nil {
		err = ctx.Err()
	}
	if err == nil {
		job.SHA256, job.Size = manifest.SHA256, manifest.Size
		err = manager.persist(job.File+".manifest", manifest)
	}
	if err == nil {
		err = os.Rename(temporary, job.File)
	}
	if err != nil {
		_ = os.Remove(temporary)
		_ = os.Remove(job.File + ".manifest")
	}
	if err != nil {
		job.State, job.Error = "failed", err.Error()
	} else {
		job.State, job.ReadyOffline = "ready", true
	}
	if saveErr := manager.save(job); saveErr != nil {
		_ = os.Remove(job.File)
		job.State, job.ReadyOffline, job.Error = "failed", false, "download state could not be saved"
	}
	return job
}
