package backup

import (
	"testing"

	"github.com/MikeO7/kinosail/packages/servertest"
)

func TestBackupAdapterContract(t *testing.T) {
	servertest.BackupAdapterContract(t, WriteEncrypted, VerifyAuto, Command)
}
