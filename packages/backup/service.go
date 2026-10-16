package backup

import (
	"crypto/rand"
	"errors"
	"io"
	"path/filepath"
	"regexp"
)

const databaseDocumentLimit = 16 << 20

var (
	databaseDocuments = []string{"api_keys.json", "collections.json", "history.json", "lists.json", "metadata.json", "playback_markers.json", "playlist_order.json", "playlists.json", "profiles.json", "progress.json", "sessions.json", "settings.json", "smart_playlists.json"}
	versionPattern    = regexp.MustCompile(`^(dev|[A-Za-z0-9][A-Za-z0-9._+-]{0,127})$`)
)

type Database interface {
	Export() (map[string][]byte, error)
	Close() error
}

type Exporter interface {
	Export() (map[string][]byte, error)
}

type liveDatabase struct{ Exporter }

func (liveDatabase) Close() error { return nil }

type Config struct {
	DatabaseFilename string
	OpenDatabase     func(string) (Database, error)
	ValidateSettings func([]byte) error
}

type Service struct {
	databaseFilename string
	openDatabase     func(string) (Database, error)
	validateSettings func([]byte) error
	random           io.Reader
	restore          restoreOperations
}

func New(config Config) (*Service, error) {
	if config.DatabaseFilename == "" || config.DatabaseFilename == "." || config.DatabaseFilename == ".." || filepath.Base(config.DatabaseFilename) != config.DatabaseFilename || config.OpenDatabase == nil || config.ValidateSettings == nil {
		return nil, errors.New("invalid backup configuration")
	}
	return &Service{databaseFilename: config.DatabaseFilename, openDatabase: config.OpenDatabase, validateSettings: config.ValidateSettings, random: rand.Reader, restore: defaultRestoreOperations()}, nil
}

func (service *Service) WithLiveDatabase(database Exporter) *Service {
	live := *service
	live.openDatabase = func(string) (Database, error) { return liveDatabase{database}, nil }
	return &live
}

func MustNew(config Config) *Service {
	service, err := New(config)
	if err != nil {
		panic(err)
	}
	return service
}

func validVersion(version string) bool {
	return versionPattern.MatchString(version)
}
