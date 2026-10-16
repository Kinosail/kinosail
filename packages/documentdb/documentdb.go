package documentdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/ncruces/go-sqlite3/driver"
)

const (
	Filename          = "kinosail.db"
	SchemaVersion     = 3
	DocumentSizeLimit = 16 << 20
)

type Store struct {
	db, reader               *sql.DB
	upsertEntry, deleteEntry *sql.Stmt
	path                     string
	allowed, durable         map[string]struct{}
	config                   Config
}

func Open(dataDir string, keepOpen bool, config Config) (*Store, error) {
	return OpenContext(context.Background(), dataDir, keepOpen, config)
}

func OpenContext(ctx context.Context, dataDir string, keepOpen bool, config Config) (*Store, error) {
	return openContext(ctx, dataDir, keepOpen, config, systemDependencies())
}

func openContext(ctx context.Context, dataDir string, keepOpen bool, config Config, dependencies dependencies) (*Store, error) {
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
	store, err := newStore(path, keepOpen, config, allowed, dependencies)
	if err != nil {
		return nil, err
	}
	if err := store.prepare(ctx, legacy, databaseExists, dependencies); err != nil {
		_ = store.Close()
		return nil, err
	}
	for _, name := range legacy.names() {
		if err := dependencies.remove(filepath.Join(dataDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = store.Close()
			return nil, fmt.Errorf("remove imported %s: %w", name, err)
		}
	}
	return store, nil
}

func newStore(path string, keepOpen bool, config Config, allowed map[string]struct{}, dependencies dependencies) (*Store, error) {
	db, err := openDatabase(path, keepOpen, dependencies)
	if err != nil {
		return nil, err
	}
	reader, err := openReader(path, keepOpen, dependencies)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	durable := make(map[string]struct{}, len(config.DurableDocuments))
	for _, name := range config.DurableDocuments {
		durable[name] = struct{}{}
	}
	return &Store{db: db, reader: reader, path: path, allowed: allowed, durable: durable, config: config}, nil
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

func (store *Store) prepare(ctx context.Context, legacy legacyState, databaseExists bool, dependencies dependencies) error {
	if databaseExists && store.config.ValidateBeforeMigration {
		if err := store.validateExisting(ctx); err != nil {
			return err
		}
	}
	if err := store.initialize(ctx, legacy); err != nil {
		return err
	}
	if err := store.prepareStatements(ctx); err != nil {
		return err
	}
	if _, err := store.ExportContext(ctx); err != nil {
		return err
	}
	return dependencies.chmod(store.path, 0o600)
}

func (store *Store) prepareStatements(ctx context.Context) error {
	var err error
	if store.upsertEntry, err = store.db.PrepareContext(ctx, upsertEntrySQL); err != nil {
		return err
	}
	store.deleteEntry, err = store.db.PrepareContext(ctx, deleteEntrySQL)
	return err
}

func (store *Store) validDocument(name string, data []byte) bool {
	return json.Valid(data) && store.acceptable(name, data, DocumentSizeLimit)
}

func (store *Store) acceptable(name string, data []byte, limit int) bool {
	_, ok := store.allowed[name]
	return ok && len(data) <= limit && (store.config.Validate == nil || store.config.Validate(name, data) == nil)
}

func (store *Store) validStored(document storedDocument) bool {
	if document.entries {
		return store.acceptable(document.name, document.data, EntryDocumentSizeLimit)
	}
	return store.validDocument(document.name, document.data)
}

func (store *Store) Close() error {
	if store == nil {
		return nil
	}
	var errs []error
	for _, statement := range []*sql.Stmt{store.upsertEntry, store.deleteEntry} {
		if statement != nil {
			errs = append(errs, statement.Close())
		}
	}
	return errors.Join(append(errs, store.reader.Close(), store.db.Close())...)
}

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

func (store *Store) Load(name string) ([]byte, bool, error) {
	if store == nil {
		return nil, false, nil
	}
	if _, ok := store.allowed[name]; !ok {
		return nil, false, fmt.Errorf("unknown state document %q", name)
	}
	var document storedDocument
	var found bool
	err := store.read(context.Background(), func(tx *sql.Tx) error {
		var err error
		document.name = name
		document.data, document.entries, found, err = readDocument(context.Background(), tx, name)
		return err
	})
	if err != nil || !found {
		return nil, false, err
	}
	if !store.validStored(document) {
		return nil, false, fmt.Errorf("invalid persisted %s", name)
	}
	return document.data, true, nil
}

func (store *Store) SaveJSON(name string, value any) error {
	if store == nil {
		return nil
	}
	return store.SaveJSONBatchContext(context.Background(), map[string]any{name: value})
}

func (store *Store) SaveJSONBatch(values map[string]any) error {
	return store.SaveJSONBatchContext(context.Background(), values)
}

func (store *Store) SaveJSONBatchContext(ctx context.Context, values map[string]any) error {
	if store == nil {
		return nil
	}
	return store.Commit(ctx, Batch{Documents: values})
}

func (store *Store) Export() (map[string][]byte, error) {
	return store.ExportContext(context.Background())
}

func (store *Store) ExportContext(ctx context.Context) (map[string][]byte, error) {
	result := make(map[string][]byte)
	if store == nil {
		return result, nil
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	err := store.read(ctx, func(tx *sql.Tx) error {
		if err := validateEntries(ctx, tx); err != nil {
			return err
		}
		documents, err := readDocuments(ctx, tx, true)
		if err != nil {
			return err
		}
		for _, document := range documents {
			if !store.validStored(document) {
				return errInvalidState
			}
			result[document.name] = document.data
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
