package server

import (
	"context"
	"time"

	appbackup "github.com/MikeO7/kinosail-player/internal/backup"
	"github.com/MikeO7/kinosail-player/internal/database"
	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
	"github.com/MikeO7/kinosail/packages/workload"
)

type backupManager = sharedbackup.Manager

func newBackupManager(ctx context.Context, dataDir, directory, key string, interval time.Duration, retention int, workloads *workload.Governor, stateDB *database.Store) *backupManager {
	return sharedbackup.NewAutomaticManager(ctx, dataDir, directory, key, interval, retention, func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Background) }, appbackup.LiveWriteEncrypted(stateDB), appbackup.VerifyAuto)
}
