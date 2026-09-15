// Package database owns Kinosail Dashboard's embedded SQLite state.
package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const (
	Filename          = "kinosail-dashboard.db"
	SchemaVersion     = 1
	DocumentSizeLimit = 16 << 20
)

var documents = []string{"board.json", "owner.json", "sessions.json", "supporter.json"}

// Store persists bounded JSON documents in one transactional database.
type Store struct {
	db   *sql.DB
	path string
	lock *os.File
}

// Open creates or validates the application database.
func Open(ctx context.Context, dataDir string) (*Store, error) {
	return openStore(ctx, dataDir, storeFiles{lstat: os.Lstat, chmod: os.Chmod})
}

type storeFiles struct {
	lstat func(string) (os.FileInfo, error)
	chmod func(string, os.FileMode) error
}

func openStore(ctx context.Context, dataDir string, files storeFiles) (*Store, error) {
	if ctx == nil || stringsTrimSpace(dataDir) == "" {
		return nil, errors.New("data directory is required")
	}
	enforcePrivateCreationMask()
	path, err := files.prepare(dataDir)
	if err != nil {
		return nil, err
	}
	lock, err := acquireProcessLock(filepath.Join(dataDir, ".kinosail-dashboard.lock"))
	if err != nil {
		return nil, err
	}
	dsn := (&url.URL{Scheme: "file", Path: path, RawQuery: "_txlock=immediate"}).String()
	// The registered driver accepts this URL-encoded DSN with its fixed, valid options.
	// Connection and filesystem errors are reported by initialize, not sql.Open.
	db, _ := sql.Open("sqlite3", dsn)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db, path: path, lock: lock}
	if err := store.initialize(ctx); err != nil {
		_ = store.Close()
		return nil, err
	}
	if err := files.chmod(path, 0o600); err != nil {
		_ = store.Close()
		return nil, err
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := files.chmod(path+suffix, 0o600); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = store.Close()
			return nil, err
		}
	}
	return store, nil
}

func (files storeFiles) prepare(dataDir string) (string, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", err
	}
	info, err := files.lstat(dataDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("invalid data directory")
	}
	if err := files.chmod(dataDir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dataDir, Filename)
	if info, err := files.lstat(path); err == nil && !info.Mode().IsRegular() {
		return "", errors.New("invalid SQLite database file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return path, nil
}

func (store *Store) initialize(ctx context.Context) error {
	var version int
	if err := store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read SQLite schema version: %w", err)
	}
	if version < 0 || version > SchemaVersion {
		return fmt.Errorf("unsupported SQLite schema version %d", version)
	}
	if _, err := store.db.ExecContext(ctx, `PRAGMA journal_mode=WAL; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000;`); err != nil {
		return fmt.Errorf("initialize SQLite: %w", err)
	}
	if version == 0 {
		if _, err := store.db.ExecContext(ctx, `CREATE TABLE state (
			name TEXT PRIMARY KEY NOT NULL,
			value BLOB NOT NULL CHECK(length(value) <= 16777216)
		) STRICT;
		PRAGMA user_version=1;`); err != nil {
			return fmt.Errorf("initialize SQLite schema: %w", err)
		}
	}
	var check string
	if err := store.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&check); err != nil || check != "ok" {
		return fmt.Errorf("SQLite integrity check failed: %s: %w", check, err)
	}
	return nil
}

// Close releases database resources.
func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	var databaseErr error
	if store.db != nil {
		databaseErr = store.db.Close()
		store.db = nil
	}
	lockErr := releaseProcessLock(store.lock)
	store.lock = nil
	return errors.Join(databaseErr, lockErr)
}

// Path returns the database path for backup and diagnostics.
func (store *Store) Path() string { return store.path }

// LoadJSON decodes one allowlisted document.
func (store *Store) LoadJSON(ctx context.Context, name string, target any) (bool, error) {
	if ctx == nil || !slices.Contains(documents, name) {
		return false, errors.New("invalid state document")
	}
	var data []byte
	err := store.db.QueryRowContext(ctx, `SELECT value FROM state WHERE name = ?`, name).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(data) > DocumentSizeLimit || !json.Valid(data) {
		return false, fmt.Errorf("invalid persisted %s", name)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("decode %s: %w", name, err)
	}
	return true, nil
}

// SaveJSON atomically replaces one allowlisted document.
func (store *Store) SaveJSON(ctx context.Context, name string, value any) error {
	return store.SaveJSONBatch(ctx, map[string]any{name: value})
}

// SaveJSONBatch validates then commits related documents in one transaction.
func (store *Store) SaveJSONBatch(ctx context.Context, values map[string]any) error {
	if ctx == nil || len(values) == 0 || len(values) > len(documents) {
		return errors.New("invalid state document batch")
	}
	encoded, err := encodeDocuments(values)
	if err != nil {
		return err
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for name, data := range encoded {
		if _, err := tx.ExecContext(ctx, `INSERT INTO state(name, value) VALUES(?, ?)
			ON CONFLICT(name) DO UPDATE SET value = excluded.value`, name, data); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func encodeDocuments(values map[string]any) (map[string][]byte, error) {
	encoded := make(map[string][]byte, len(values))
	for name, value := range values {
		if !slices.Contains(documents, name) {
			return nil, errors.New("invalid state document")
		}
		data, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		if len(data) > DocumentSizeLimit || !json.Valid(data) {
			return nil, errors.New("state document exceeds its limit")
		}
		encoded[name] = data
	}
	return encoded, nil
}

func stringsTrimSpace(value string) string {
	for len(value) > 0 && (value[0] == ' ' || value[0] == '\t' || value[0] == '\r' || value[0] == '\n') {
		value = value[1:]
	}
	return value
}
