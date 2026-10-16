package backup

import (
	"io"

	"github.com/MikeO7/kinosail-player/internal/database"
	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

var ApplicationVersion = "dev"

var service = sharedbackup.MustNew(sharedbackup.Config{
	DatabaseFilename: database.Filename,
	OpenDatabase: func(dataDir string) (sharedbackup.Database, error) {
		return database.Open(dataDir, false)
	},
	ValidateSettings: func([]byte) error { return nil },
})

var Command = sharedbackup.BindCommand(service, func() string { return ApplicationVersion })

func Write(writer io.Writer, dataDir string) error {
	return service.Write(writer, dataDir, ApplicationVersion)
}

func WriteEncrypted(writer io.Writer, dataDir, passphrase string) error {
	return service.WriteEncrypted(writer, dataDir, passphrase, ApplicationVersion)
}

func LiveWrite(store *database.Store) func(io.Writer, string) error {
	live := liveService(store)
	return func(writer io.Writer, dataDir string) error { return live.Write(writer, dataDir, ApplicationVersion) }
}

func LiveWriteEncrypted(store *database.Store) func(io.Writer, string, string) error {
	live := liveService(store)
	return func(writer io.Writer, dataDir, passphrase string) error {
		return live.WriteEncrypted(writer, dataDir, passphrase, ApplicationVersion)
	}
}

func liveService(store *database.Store) *sharedbackup.Service {
	if store == nil {
		return service
	}
	return service.WithLiveDatabase(store)
}

func VerifyAuto(reader io.Reader, passphrase string) error {
	return service.VerifyAuto(reader, passphrase)
}
