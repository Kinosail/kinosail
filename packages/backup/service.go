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

// Database provides a consistent logical snapshot of Kinosail state.
type Database interface {
	Export() (map[string][]byte, error)
	Close() error
}

// Config supplies the app-owned SQLite adapter and settings validation rule.
type Config struct {
	DatabaseFilename string
	OpenDatabase     func(string) (Database, error)
	ValidateSettings func([]byte) error
}

// Service archives and restores one Kinosail app's installation state.
type Service struct {
	databaseFilename string
	openDatabase     func(string) (Database, error)
	validateSettings func([]byte) error
	random           io.Reader
	restore          restoreOperations
}

// New validates app-specific dependencies without touching installation state.
func New(config Config) (*Service, error) {
	if config.DatabaseFilename == "" || config.DatabaseFilename == "." || config.DatabaseFilename == ".." || filepath.Base(config.DatabaseFilename) != config.DatabaseFilename || config.OpenDatabase == nil || config.ValidateSettings == nil {
		return nil, errors.New("invalid backup configuration")
	}
	return &Service{databaseFilename: config.DatabaseFilename, openDatabase: config.OpenDatabase, validateSettings: config.ValidateSettings, random: rand.Reader, restore: defaultRestoreOperations()}, nil
}

// MustNew constructs a service from compile-time app dependencies.
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
