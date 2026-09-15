// Package backup adapts Player's state database to the shared archive service.
package backup

import (
	"io"

	"github.com/MikeO7/kinosail-player/internal/database"
	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

// ApplicationVersion is set by the executable before producing recovery archives.
var ApplicationVersion = "dev"

var service = sharedbackup.MustNew(sharedbackup.Config{
	DatabaseFilename: database.Filename,
	OpenDatabase: func(dataDir string) (sharedbackup.Database, error) {
		return database.Open(dataDir, false)
	},
	ValidateSettings: func([]byte) error { return nil },
})

// Command runs Player's backup and restore command contract.
var Command = sharedbackup.BindCommand(service, func() string { return ApplicationVersion })

func Write(writer io.Writer, dataDir string) error {
	return service.Write(writer, dataDir, ApplicationVersion)
}

func WriteEncrypted(writer io.Writer, dataDir, passphrase string) error {
	return service.WriteEncrypted(writer, dataDir, passphrase, ApplicationVersion)
}

func VerifyAuto(reader io.Reader, passphrase string) error {
	return service.VerifyAuto(reader, passphrase)
}
