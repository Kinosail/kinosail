package backup_test

import (
	"errors"
	"io"

	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

var testArchive = archiveHarness{sharedbackup.MustNew(sharedbackup.Config{
	DatabaseFilename: "kinosail.db",
	OpenDatabase: func(string) (sharedbackup.Database, error) {
		return nil, errors.New("unexpected test database")
	},
	ValidateSettings: func([]byte) error { return nil },
})}

type archiveHarness struct {
	service *sharedbackup.Service
}

func (archive archiveHarness) Write(writer io.Writer, dataDir string) error {
	return archive.service.Write(writer, dataDir, "dev")
}

func (archive archiveHarness) Restore(reader io.Reader, dataDir string) error {
	return archive.service.Restore(reader, dataDir)
}

func (archive archiveHarness) WriteAuto(writer io.Writer, dataDir, passphrase string) error {
	return archive.service.WriteAuto(writer, dataDir, passphrase, "dev")
}

func (archive archiveHarness) WriteEncrypted(writer io.Writer, dataDir, passphrase string) error {
	return archive.service.WriteEncrypted(writer, dataDir, passphrase, "dev")
}

func (archive archiveHarness) RestoreEncrypted(reader io.Reader, dataDir, passphrase string) error {
	return archive.service.RestoreEncrypted(reader, dataDir, passphrase)
}

func (archive archiveHarness) VerifyEncrypted(reader io.Reader, passphrase string) error {
	return archive.service.VerifyEncrypted(reader, passphrase)
}

func (archive archiveHarness) VerifyAuto(reader io.Reader, passphrase string) error {
	return archive.service.VerifyAuto(reader, passphrase)
}

func (archive archiveHarness) RestoreAuto(reader io.Reader, dataDir, passphrase string) error {
	return archive.service.RestoreAuto(reader, dataDir, passphrase)
}
