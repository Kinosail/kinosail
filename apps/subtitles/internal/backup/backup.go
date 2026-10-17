package backup

import (
	"io"

	"github.com/MikeO7/kinosail-subtitles/internal/database"
	"github.com/MikeO7/kinosail-subtitles/internal/settingsstate"
	sharedbackup "github.com/MikeO7/kinosail/packages/backup"
)

var ApplicationVersion = "dev"

var service = sharedbackup.MustNew(sharedbackup.Config{
	DatabaseFilename: database.Filename,
	Documents:        database.Documents,
	EntryDocuments:   database.EntryDocuments,
	OpenDatabase: func(dataDir string) (sharedbackup.Database, error) {
		return database.Open(dataDir, false)
	},
	ValidateSettings: settingsstate.Validate,
})

var Command = sharedbackup.BindCommand(service, func() string { return ApplicationVersion })

func Write(writer io.Writer, dataDir string) error {
	return service.Write(writer, dataDir, ApplicationVersion)
}

func WriteEncrypted(writer io.Writer, dataDir, passphrase string) error {
	return service.WriteEncrypted(writer, dataDir, passphrase, ApplicationVersion)
}

func LiveWriteEncrypted(store *database.Store) func(io.Writer, string, string) error {
	live := service
	if store != nil {
		live = service.WithLiveDatabase(store)
	}
	return func(writer io.Writer, dataDir, passphrase string) error {
		return live.WriteEncrypted(writer, dataDir, passphrase, ApplicationVersion)
	}
}

func VerifyAuto(reader io.Reader, passphrase string) error {
	return service.VerifyAuto(reader, passphrase)
}
