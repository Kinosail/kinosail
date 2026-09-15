package server_test

import (
	"net/http"
	"testing"

	"github.com/MikeO7/kinosail-player/internal/server"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestAutomaticBackupsFailClosedWithoutEncryptionKey(t *testing.T) {
	servertest.AssertAutomaticBackupsFailClosed(t, backupAPIFixture())
}

func TestOwnerCanRunEncryptedBackupThroughAPIAndWeb(t *testing.T) {
	servertest.AssertEncryptedBackupAPIAndWeb(t, backupAPIFixture())
}

func backupAPIFixture() servertest.BackupAPIFixture {
	return servertest.BackupAPIFixture{
		New: func(config servertest.BackupAPIConfig) http.Handler {
			return server.New(server.Config{Lifecycle: config.Lifecycle, DataDir: config.DataDir, BackupDir: config.BackupDir, BackupKey: config.Key, BackupInterval: config.Interval, RequireAuth: true})
		},
		SignIn: signInTestProfile, Get: getWithCookie, Web: requestWithCookie,
	}
}
