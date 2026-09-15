package backup

import (
	"testing"

	"github.com/MikeO7/kinosail-player/internal/database"
	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestSQLiteBackupContract(t *testing.T) {
	servertest.SQLiteBackupContract(t, servertest.SQLiteBackupFixture[*database.Store]{
		Filename: database.Filename, Open: database.Open, Write: Write, Restore: service.Restore,
	})
}
