package server

import (
	"context"
	"time"

	sharedmaintenance "github.com/MikeO7/kinosail/packages/maintenance"
)

type maintenanceManager = sharedmaintenance.Manager

func newMaintenanceManager(ctx context.Context, hls *hlsManager, backups *backupManager, metadata *metadataStore, markers *markerAnalyzer, interval time.Duration, limit int64) *maintenanceManager {
	return sharedmaintenance.NewApplication(ctx, interval, limit, hls.cacheOps, backups, metadata.pruneExpiredArtwork, metadata.configured, markers)
}
