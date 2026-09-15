package documentdb

import (
	"context"
	"encoding/json"
	"errors"
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
	query := `SELECT name, value FROM state ORDER BY name`
	if version >= 2 {
		if err := store.validateEntries(ctx); err != nil {
			return err
		}
		query = `SELECT name, ` + logicalValue + ` FROM state ORDER BY name`
	}
	rows, err := store.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var data []byte
		if err := rows.Scan(&name, &data); err != nil {
			return errors.New("database contains invalid state")
		}
		_, isRetired := retired[name]
		if !store.validStoredDocument(name, data, isRetired) {
			return errors.New("database contains invalid state")
		}
	}
	return rows.Err()
}

func (store *Store) initialize(ctx context.Context, legacy map[string][]byte) error { //nolint:cyclop,gocognit // Schema, integrity, cleanup, and import share one startup transaction.
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
	if version < 2 {
		if _, err := tx.ExecContext(ctx, entrySchema); err != nil {
			return fmt.Errorf("upgrade record storage: %w", err)
		}
	}
	for _, name := range store.config.RetiredDocuments {
		if _, err := tx.ExecContext(ctx, `DELETE FROM state_entries WHERE name = ?`, name); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM state WHERE name = ?`, name); err != nil {
			return fmt.Errorf("remove retired state %s: %w", name, err)
		}
	}
	for name, data := range legacy {
		if err := store.saveDocument(ctx, tx, name, data); err != nil {
			return fmt.Errorf("import %s: %w", name, err)
		}
	}
	return tx.Commit()
}

func (store *Store) validStoredDocument(name string, data []byte, retired bool) bool {
	if retired {
		return len(data) <= DocumentSizeLimit && json.Valid(data)
	}
	return store.validDocument(name, data)
}
