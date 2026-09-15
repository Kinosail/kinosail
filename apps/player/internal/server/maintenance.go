package server

import (
	"context"
	"net/http"
	"time"

	sharedmaintenance "github.com/MikeO7/kinosail/packages/maintenance"
)

type maintenanceStatus = sharedmaintenance.Status

type maintenanceManager struct{ *sharedmaintenance.Manager }

func newMaintenanceManager(ctx context.Context, hls *hlsManager, backups *backupManager, metadata *metadataStore, markers *markerAnalyzer, interval time.Duration, limit int64) *maintenanceManager {
	manager := &maintenanceManager{sharedmaintenance.New(ctx, interval, limit, sharedmaintenance.Dependencies{
		PruneCache: hls.cacheOps.Prune, PruneMetadata: metadata.pruneExpiredArtwork, CacheStats: hls.cacheOps.Stats,
		BackupStatus: func() sharedmaintenance.BackupStatus {
			status := backups.Status()
			return sharedmaintenance.BackupStatus{Enabled: status.Enabled, Encrypted: status.Encrypted, LastError: status.LastError}
		},
		MetadataConfigured: metadata.configured,
		AnalysisStatus:     func() string { state, _, _ := markers.Status(); return state },
	})}
	markers.SetWait(manager.WaitIdle)
	return manager
}

func (manager *maintenanceManager) status() maintenanceStatus { return manager.Status() }
func (manager *maintenanceManager) track(next http.Handler) http.Handler {
	return manager.Track(next)
}

func (manager *maintenanceManager) serveStatus(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, manager.Status(), http.StatusOK)
}
