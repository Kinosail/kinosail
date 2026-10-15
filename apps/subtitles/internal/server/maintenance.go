package server

import (
	"context"
	"time"

	sharedmaintenance "github.com/MikeO7/kinosail/packages/maintenance"
)

type maintenanceManager = sharedmaintenance.Manager

func newMaintenanceManager(ctx context.Context, backups *backupManager, metadata *metadataStore, interval time.Duration) *maintenanceManager {
	return sharedmaintenance.NewSubtitles(ctx, interval, backups, metadata.pruneExpiredArtwork, metadata.configured)
}
