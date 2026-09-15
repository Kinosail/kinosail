// Package documentdb owns Kinosail's embedded SQLite document store.
package documentdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const (
	Filename          = "kinosail.db"
	SchemaVersion     = 2
	DocumentSizeLimit = 16 << 20
)

// Store persists bounded JSON documents in one transactional database.
type Store struct {
	db      *sql.DB
	path    string
	allowed map[string]struct{}
	config  Config
}

// Open initializes the database and atomically imports legacy JSON files.
func Open(dataDir string, keepOpen bool, config Config) (*Store, error) { //nolint:contextcheck // Offline import and test callers have no request lifecycle.
	return OpenContext(context.Background(), dataDir, keepOpen, config)
}

// OpenContext initializes the database within the application lifecycle.
func OpenContext(ctx context.Context, dataDir string, keepOpen bool, config Config) (*Store, error) { //nolint:gocognit,cyclop,funlen // Startup validates each persisted state boundary before migration.
	return openContext(ctx, dataDir, keepOpen, config, systemDependencies())
}

func openContext(ctx context.Context, dataDir string, keepOpen bool, config Config, dependencies dependencies) (*Store, error) { //nolint:cyclop // Ordered database initialization remains below the repository complexity ceiling.
	if dataDir == "" {
		return nil, nil
	}
	config, allowed, err := validateConfig(config)
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	legacy, err := readLegacy(dataDir, config, allowed)
	if err != nil {
		return nil, err
	}
	if err := dependencies.mkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, Filename)
	databaseExists, err := databaseFileExists(path, dependencies)
	if err != nil {
		return nil, err
	}
	db, err := openDatabase(path, keepOpen, dependencies)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, path: path, allowed: allowed, config: config}
	if err := store.prepare(ctx, legacy, databaseExists, dependencies); err != nil {
		_ = db.Close()
		return nil, err
	}
	for name := range legacy {
		if err := dependencies.remove(filepath.Join(dataDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = db.Close()
			return nil, fmt.Errorf("remove imported %s: %w", name, err)
		}
	}
	return store, nil
}

func databaseFileExists(path string, dependencies dependencies) (bool, error) {
	info, err := dependencies.lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return false, errors.New("invalid SQLite database file")
	}
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func openDatabase(path string, keepOpen bool, dependencies dependencies) (*sql.DB, error) {
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "_txlock=immediate"}).String()
	db, err := dependencies.openDB("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if keepOpen {
		db.SetMaxIdleConns(1)
	} else {
		db.SetMaxIdleConns(0)
	}
	return db, nil
}

func (store *Store) prepare(ctx context.Context, legacy map[string][]byte, databaseExists bool, dependencies dependencies) error {
	if databaseExists && store.config.ValidateBeforeMigration {
		if err := store.validateExisting(ctx); err != nil {
			return err
		}
	}
	if err := store.initialize(ctx, legacy); err != nil {
		return err
	}
	if _, err := store.ExportContext(ctx); err != nil {
		return err
	}
	return dependencies.chmod(store.path, 0o600)
}

func (store *Store) validDocument(name string, data []byte) bool {
	if _, ok := store.allowed[name]; !ok || len(data) > DocumentSizeLimit || !json.Valid(data) {
		return false
	}
	return store.config.Validate == nil || store.config.Validate(name, data) == nil
}

// Close releases database resources.
func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	return store.db.Close()
}

// LoadJSON decodes one known document. Missing documents leave target unchanged.
func (store *Store) LoadJSON(name string, target any) (bool, error) {
	data, found, err := store.Load(name)
	if err != nil || !found {
		return found, err
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("decode %s: %w", name, err)
	}
	return true, nil
}

// Load returns a validated copy of one known document.
func (store *Store) Load(name string) ([]byte, bool, error) {
	if store == nil {
		return nil, false, nil
	}
	if _, ok := store.allowed[name]; !ok {
		return nil, false, fmt.Errorf("unknown state document %q", name)
	}
	var data []byte
	err := store.db.QueryRowContext(context.Background(), `SELECT `+logicalValue+` FROM state WHERE name = ?`, name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !store.validDocument(name, data) {
		return nil, false, fmt.Errorf("invalid persisted %s", name)
	}
	return data, true, nil
}

// SaveJSON atomically replaces one known document.
func (store *Store) SaveJSON(name string, value any) error {
	if store == nil {
		return nil
	}
	return store.SaveJSONBatchContext(context.Background(), map[string]any{name: value})
}

// SaveJSONBatch validates then commits related documents in one transaction.
func (store *Store) SaveJSONBatch(values map[string]any) error { //nolint:contextcheck // Store adapters share this bounded local durability operation outside transport lifetimes.
	return store.SaveJSONBatchContext(context.Background(), values)
}

// SaveJSONBatchContext validates then commits related documents in one transaction.
func (store *Store) SaveJSONBatchContext(ctx context.Context, values map[string]any) error {
	if store == nil {
		return nil
	}
	if ctx == nil || len(values) > len(store.allowed) {
		return errors.New("invalid state document batch")
	}
	documents := make(map[string][]byte, len(values))
	for name, value := range values {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if !store.validDocument(name, data) {
			return fmt.Errorf("invalid state document %q", name)
		}
		documents[name] = data
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for name, data := range documents {
		if err := store.saveDocument(ctx, tx, name, data); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Export returns a consistent snapshot of every stored logical document.
func (store *Store) Export() (map[string][]byte, error) {
	return store.ExportContext(context.Background())
}

// ExportContext returns a consistent snapshot within the caller lifecycle.
func (store *Store) ExportContext(ctx context.Context) (map[string][]byte, error) {
	result := make(map[string][]byte)
	if store == nil {
		return result, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	if err := store.validateEntries(ctx); err != nil {
		return nil, err
	}
	rows, err := store.db.QueryContext(ctx, `SELECT name, `+logicalValue+` FROM state ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var data []byte
		if err := rows.Scan(&name, &data); err != nil || !store.validDocument(name, data) {
			return nil, errors.New("database contains invalid state")
		}
		result[name] = append([]byte(nil), data...)
	}
	return result, rows.Err()
}

func readLegacy(dataDir string, config Config, allowed map[string]struct{}) (map[string][]byte, error) {
	store := &Store{allowed: allowed, config: config}
	legacy := make(map[string][]byte)
	for _, name := range config.Documents {
		path := filepath.Join(dataDir, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Size() > DocumentSizeLimit {
			return nil, fmt.Errorf("invalid legacy state %s", name)
		}
		data, err := os.ReadFile(path) //nolint:gosec // The validated name is below the installation directory.
		if err != nil || !store.validDocument(name, data) {
			return nil, fmt.Errorf("invalid legacy state %s", name)
		}
		legacy[name] = data
	}
	return legacy, nil
}
