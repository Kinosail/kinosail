package server

import (
	"context"
	"time"

	appbackup "github.com/MikeO7/kinosail-subtitles/internal/backup"
	"github.com/MikeO7/kinosail-subtitles/internal/database"
	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
	"github.com/MikeO7/kinosail/packages/workload"
)

type backupManager = sharedbackup.Manager

func newBackupManager(ctx context.Context, dataDir, directory, key string, interval time.Duration, retention int, workloads *workload.Governor, databases ...*database.Store) *backupManager {
	return sharedbackup.NewAutomaticManager(ctx, dataDir, directory, key, interval, retention, func(ctx context.Context) (func(), error) { return workloads.Acquire(ctx, workload.Background) }, appbackup.LiveWriteEncrypted(configuredDatabase(databases)), appbackup.VerifyAuto)
}
