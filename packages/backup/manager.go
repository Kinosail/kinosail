package backup

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ManagerConfig supplies app archive adapters and backup policy.
type ManagerConfig struct {
	Context        context.Context
	DataDir        string
	Directory      string
	Key            string
	Retention      int
	Interval       time.Duration
	Acquire        func(context.Context) (func(), error)
	WriteEncrypted func(io.Writer, string, string) error
	VerifyAuto     func(io.Reader, string) error
}

// Manager owns automatic encrypted backups, retention, and verification.
type Manager struct {
	mu                      sync.Mutex
	dataDir, directory, key string
	retention               int
	interval                time.Duration
	lastSuccess, lastVerify time.Time
	lastError               string
	acquire                 func(context.Context) (func(), error)
	writeEncrypted          func(io.Writer, string, string) error
	verifyAuto              func(io.Reader, string) error
	readDirectory           func(string) ([]os.DirEntry, error)
	remove                  func(string) error
}

// NewManager constructs a backup manager and starts its optional schedule.
func NewManager(config ManagerConfig) *Manager {
	manager := &Manager{
		dataDir: config.DataDir, directory: config.Directory, key: config.Key,
		retention: config.Retention, interval: config.Interval, acquire: config.Acquire,
		writeEncrypted: config.WriteEncrypted, verifyAuto: config.VerifyAuto,
		readDirectory: os.ReadDir, remove: os.Remove,
	}
	if manager.retention <= 0 {
		manager.retention = 7
	}
	if config.Context != nil && manager.directory != "" && manager.interval > 0 {
		go manager.schedule(config.Context)
	}
	return manager
}

// NewAutomaticManager adapts an app archive implementation to the shared manager.
func NewAutomaticManager(ctx context.Context, dataDir, directory, key string, interval time.Duration, retention int, acquire func(context.Context) (func(), error), write func(io.Writer, string, string) error, verify func(io.Reader, string) error) *Manager { //nolint:lll // A single factory keeps app adapters declarative.
	return NewManager(ManagerConfig{
		Context: ctx, DataDir: dataDir, Directory: directory, Key: key,
		Retention: retention, Interval: interval, Acquire: acquire,
		WriteEncrypted: write, VerifyAuto: verify,
	})
}

func (manager *Manager) schedule(ctx context.Context) {
	delay := min(manager.interval, 250*time.Millisecond)
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if err := manager.Write(ctx); err != nil {
			slog.Warn("automatic backup failed", "error", err)
			delay = manager.retryDelay()
		} else {
			slog.Info("automatic backup completed", "encrypted", manager.key != "", "retention", manager.retention)
			delay = manager.interval
		}
	}
}

func (manager *Manager) retryDelay() time.Duration {
	if manager.validate() != nil {
		return manager.interval
	}
	return min(manager.interval, time.Minute)
}

// Write waits for background capacity and writes one backup.
func (manager *Manager) Write(ctx context.Context) error {
	if err := manager.validate(); err != nil {
		manager.recordError(err)
		return err
	}
	release, err := manager.reserve(ctx)
	if err != nil {
		return err
	}
	defer release()
	return manager.WriteNow()
}

func (manager *Manager) reserve(ctx context.Context) (func(), error) {
	if manager.acquire == nil {
		return func() {}, nil
	}
	return manager.acquire(ctx)
}

// WriteNow creates, verifies, and retains one encrypted backup immediately.
func (manager *Manager) WriteNow() error {
	if err := manager.validate(); err != nil {
		manager.recordError(err)
		return err
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	err := manager.writeLocked()
	if err == nil {
		manager.lastSuccess = time.Now().UTC()
		err = manager.verifyLocked()
	}
	if err != nil {
		manager.lastError = err.Error()
	}
	return err
}

func (manager *Manager) recordError(err error) {
	manager.mu.Lock()
	manager.lastError = err.Error()
	manager.mu.Unlock()
}

func (manager *Manager) writeLocked() error { //nolint:cyclop,gocognit // Archive creation, atomic replacement, and retention fail closed in one operation.
	if err := manager.validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(manager.directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(manager.directory, ".backup-*")
	if err == nil {
		defer func() { _ = os.Remove(temporary.Name()) }()
		err = manager.writeEncrypted(temporary, manager.dataDir, manager.key)
	}
	if temporary != nil {
		if err == nil {
			err = temporary.Sync()
		}
		if closeErr := temporary.Close(); err == nil {
			err = closeErr
		}
	}
	if err == nil {
		err = os.Rename(temporary.Name(), filepath.Join(manager.directory, "kinosail-"+time.Now().UTC().Format("20060102T150405.000000000Z")+".backup"))
	}
	if err != nil {
		return err
	}
	files, err := manager.Files()
	if err != nil {
		return err
	}
	for len(files) > manager.retention {
		if err := manager.remove(files[0]); err != nil {
			return err
		}
		files = files[1:]
	}
	return nil
}

func (manager *Manager) validate() error {
	if manager.directory == "" || manager.dataDir == "" {
		return errors.New("backups are not configured")
	}
	if manager.key == "" {
		return errors.New("backup encryption key is required")
	}
	if manager.writeEncrypted == nil || manager.verifyAuto == nil {
		return errors.New("backup implementation is not configured")
	}
	return nil
}

// Status describes the current automatic backup state.
type Status struct {
	Enabled      bool   `json:"enabled"`
	Encrypted    bool   `json:"encrypted"`
	Directory    string `json:"directory,omitempty"`
	Latest       string `json:"latest,omitempty"`
	Retention    int    `json:"retention"`
	Interval     string `json:"interval,omitempty"`
	LastSuccess  string `json:"lastSuccess,omitempty"`
	LastVerified string `json:"lastVerified,omitempty"`
	LastError    string `json:"lastError,omitempty"`
}

// Status returns a safe snapshot of the current backup state.
func (manager *Manager) Status() Status {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	files, err := manager.Files()
	status := Status{Enabled: manager.directory != "" && manager.dataDir != "" && manager.key != "", Encrypted: manager.key != "", Directory: manager.directory, Retention: manager.retention, Interval: manager.interval.String(), LastError: manager.lastError}
	if err != nil {
		status.LastError = err.Error()
	}
	if len(files) > 0 {
		status.Latest = filepath.Base(files[len(files)-1])
	}
	if !manager.lastSuccess.IsZero() {
		status.LastSuccess = manager.lastSuccess.Format(time.RFC3339)
	}
	if !manager.lastVerify.IsZero() {
		status.LastVerified = manager.lastVerify.Format(time.RFC3339)
	}
	return status
}

// Verify checks the newest available backup.
func (manager *Manager) Verify() error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	err := manager.verifyLocked()
	if err != nil {
		manager.lastError = err.Error()
	}
	return err
}

func (manager *Manager) verifyLocked() error {
	files, err := manager.Files()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no backup is available")
	}
	file, err := os.Open(files[len(files)-1]) //nolint:gosec // Files returns only regular entries in the configured backup directory.
	if err == nil {
		err = manager.verifyAuto(file, manager.key)
	}
	if file != nil {
		_ = file.Close()
	}
	if err != nil {
		return err
	}
	manager.lastVerify, manager.lastError = time.Now().UTC(), ""
	return nil
}

// Files returns sorted regular Kinosail backup paths from the configured directory.
func (manager *Manager) Files() ([]string, error) {
	entries, err := manager.readDirectory(manager.directory)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "kinosail-") || !strings.HasSuffix(name, ".backup") && !strings.HasSuffix(name, ".tar.gz") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		if info.Mode().IsRegular() {
			files = append(files, filepath.Join(manager.directory, name))
		}
	}
	sort.Strings(files)
	return files, nil
}
