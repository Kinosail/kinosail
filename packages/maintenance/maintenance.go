package maintenance

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
	"github.com/MikeO7/kinosail/packages/markers"
	"github.com/MikeO7/kinosail/packages/playback"
)

const (
	defaultCacheLimit = int64(10_737_418_240)
	pollInterval      = time.Duration(250_000_000)
)

type Dependencies struct {
	PruneCache         func(int64) (int64, error)
	PruneMetadata      func() error
	CacheStats         func() (int64, error)
	BackupStatus       func() BackupStatus
	MetadataConfigured func() bool
	AnalysisStatus     func() string
}

type Manager struct {
	dependencies Dependencies
	interval     time.Duration
	limit        int64
	now          func() time.Time
	foreground   atomic.Int64
	quietUntil   atomic.Int64
}

func New(ctx context.Context, interval time.Duration, limit int64, dependencies Dependencies) *Manager {
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	if limit <= 0 {
		limit = defaultCacheLimit
	}
	manager := &Manager{dependencies: dependencies, interval: interval, limit: limit, now: time.Now}
	if ctx != nil {
		go manager.schedule(ctx)
	}
	return manager
}

func NewBound(ctx context.Context, interval time.Duration, limit int64, dependencies Dependencies, setWait func(func(context.Context) error)) *Manager {
	manager := New(ctx, interval, limit, dependencies)
	if setWait != nil {
		setWait(manager.WaitIdle)
	}
	return manager
}

func NewApplication(ctx context.Context, interval time.Duration, limit int64, cache playback.HLSCacheControl, backups *sharedbackup.Manager, pruneMetadata func() error, metadataConfigured func() bool, analysis *markers.Analyzer) *Manager {
	return NewBound(ctx, interval, limit, Dependencies{
		PruneCache: cache.Prune, PruneMetadata: pruneMetadata, CacheStats: cache.Stats,
		BackupStatus: func() BackupStatus {
			status := backups.Status()
			return BackupStatus{Enabled: status.Enabled, Encrypted: status.Encrypted, LastError: status.LastError}
		},
		MetadataConfigured: metadataConfigured,
		AnalysisStatus:     func() string { state, _, _ := analysis.Status(); return state },
	}, analysis.SetWait)
}

func (manager *Manager) schedule(ctx context.Context) {
	delay := initialDelay(manager.interval)
	for {
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			_ = manager.Run()
			delay = manager.interval
		}
	}
}

func initialDelay(interval time.Duration) time.Duration {
	return min(interval, pollInterval)
}

func (manager *Manager) Run() error {
	if manager.Busy() {
		return nil
	}
	_, cacheErr := manager.dependencies.PruneCache(manager.limit)
	metadataErr := manager.dependencies.PruneMetadata()
	err := errors.Join(cacheErr, metadataErr)
	if err != nil {
		slog.Warn("automatic maintenance failed", "error", err)
	} else {
		slog.Debug("automatic maintenance completed")
	}
	return err
}

func (manager *Manager) Busy() bool {
	return manager.foreground.Load() != 0 || manager.now().UnixNano() < manager.quietUntil.Load()
}

func (manager *Manager) Track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || !mediaWorkPath(request.URL.Path) {
			next.ServeHTTP(writer, request)
			return
		}
		manager.foreground.Add(1)
		defer func() {
			manager.foreground.Add(-1)
			manager.quietUntil.Store(manager.now().Add(2 * time.Second).UnixNano())
		}()
		next.ServeHTTP(writer, request)
	})
}

func mediaWorkPath(path string) bool { //nolint:cyclop // Cache namespaces are enumerated explicitly to protect durable data.
	return strings.HasPrefix(path, "/media/") || strings.HasPrefix(path, "/hls/") || strings.HasPrefix(path, "/download/") ||
		strings.HasPrefix(path, "/Videos/") || strings.HasPrefix(path, "/Audio/") ||
		strings.HasPrefix(path, "/api/v1/downloads/") && strings.HasSuffix(path, "/file") ||
		strings.HasPrefix(path, "/Items/") && (strings.HasSuffix(path, "/Download") || strings.HasSuffix(path, "/File"))
}

func (manager *Manager) WaitIdle(ctx context.Context) error {
	for manager.Busy() {
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}
