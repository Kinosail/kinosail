package documentdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

func (store *Store) validateExisting(ctx context.Context) error {
	retired := make(map[string]struct{}, len(store.config.RetiredDocuments))
	for _, name := range store.config.RetiredDocuments {
		retired[name] = struct{}{}
	}
	var version int
	if err := store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version >= 2 {
		if err := validateEntries(ctx, store.db); err != nil {
			return err
		}
	}
	documents, err := readDocuments(ctx, store.db, version >= 2)
	if err != nil {
		return err
	}
	for _, document := range documents {
		_, isRetired := retired[document.name]
		if !store.validStoredDocument(document, isRetired) {
			return errInvalidState
		}
	}
	return nil
}

func (store *Store) initialize(ctx context.Context, legacy legacyState) error {
	var version int
	if err := store.db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&version); err != nil {
		return fmt.Errorf("read SQLite schema version: %w", err)
	}
	if version < 0 || version > SchemaVersion {
		return fmt.Errorf("unsupported SQLite schema version %d", version)
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
	if err := store.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&check); err != nil {
		return fmt.Errorf("SQLite integrity check failed: %w", err)
	}
	if check != "ok" {
		return fmt.Errorf("SQLite integrity check failed: %s", check)
	}
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := migrateSchema(ctx, tx, version); err != nil {
		return err
	}
	for _, name := range store.config.RetiredDocuments {
		if _, err := tx.ExecContext(ctx, `DELETE FROM state_entries WHERE name = ?`, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM state WHERE name = ?`, name); err != nil {
			return fmt.Errorf("remove retired state %s: %w", name, err)
		}
	}
	for name, data := range legacy.documents {
		if err := store.saveDocument(ctx, tx, name, data); err != nil {
			return fmt.Errorf("import %s: %w", name, err)
		}
	}
	for name, path := range legacy.entries {
		if err := store.importEntries(ctx, tx, name, path); err != nil {
			return fmt.Errorf("import %s: %w", name, err)
		}
	}
	return tx.Commit()
}

func migrateSchema(ctx context.Context, tx *sql.Tx, version int) error {
	if version < 2 {
		if _, err := tx.ExecContext(ctx, entrySchema); err != nil {
			return fmt.Errorf("upgrade record storage: %w", err)
		}
	}
	if version < 3 {
		if _, err := tx.ExecContext(ctx, recordSchema); err != nil {
			return fmt.Errorf("upgrade record limits: %w", err)
		}
	}
	return nil
}

func (store *Store) validStoredDocument(document storedDocument, retired bool) bool {
	if retired {
		limit := DocumentSizeLimit
		if document.entries {
			limit = EntryDocumentSizeLimit
		}
		return len(document.data) <= limit && json.Valid(document.data)
	}
	return store.validStored(document)
}
