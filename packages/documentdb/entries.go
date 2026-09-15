package documentdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

const entrySchema = `
ALTER TABLE state ADD COLUMN entry_mode INTEGER NOT NULL DEFAULT 0 CHECK(entry_mode IN (0,1));
ALTER TABLE state ADD COLUMN entry_bytes INTEGER NOT NULL DEFAULT 1 CHECK(entry_bytes BETWEEN 1 AND 16777216);
ALTER TABLE state ADD COLUMN entry_count INTEGER NOT NULL DEFAULT 0 CHECK(entry_count BETWEEN 0 AND 1000000);
CREATE TABLE state_entries (
    name TEXT NOT NULL, key TEXT NOT NULL CHECK(length(CAST(key AS BLOB)) BETWEEN 1 AND 256),
    value BLOB NOT NULL CHECK(length(value) <= 16777216),
    cost INTEGER NOT NULL CHECK(cost BETWEEN 1 AND 16777216),
    PRIMARY KEY(name,key)
) STRICT, WITHOUT ROWID;
PRAGMA user_version=2;`

// Export/Load retain the logical JSON document contract used by backups/imports.
const logicalValue = `CASE WHEN entry_mode = 1 THEN CASE WHEN (SELECT coalesce(sum(length(value)+length(CAST(json_quote(key) AS BLOB))+2),0) FROM state_entries WHERE state_entries.name=state.name) < 16777216 THEN CAST((SELECT coalesce(json_group_object(key,json(CAST(value AS TEXT))),'{}') FROM state_entries WHERE state_entries.name = state.name) AS BLOB) ELSE NULL END ELSE value END`

// EnableEntries atomically migrates a JSON object of independent records. Whole
// document saves and backup export keep their existing representation and semantics.
func (store *Store) EnableEntries(name string) error {
	if store == nil {
		return nil
	}
	if _, ok := store.allowed[name]; !ok {
		return errors.New("unknown record document")
	}
	ctx := context.Background()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	data, mode, err := entryDocument(ctx, tx, name)
	if err != nil {
		return err
	}
	if mode == 1 {
		return tx.Commit()
	}
	if !store.validDocument(name, data) {
		return errors.New("invalid record document")
	}
	if err := replaceEntries(ctx, tx, name, data); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveEntry validates one independent record before committing it. The enclosing
// object byte/cardinality budgets are enforced by the same transaction.
func (store *Store) SaveEntry(name, key string, value any) error {
	if store == nil {
		return nil
	}
	data, err := store.entryValue(name, key, value)
	if err != nil {
		return err
	}
	ctx := context.Background()
	tx, err := store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := writeEntry(ctx, tx, name, key, data); err != nil {
		return err
	}
	return tx.Commit()
}

func entryCost(key string, data []byte) int {
	encoded, _ := json.Marshal(key)
	return len(encoded) + len(data) + 2
}

func replaceEntries(ctx context.Context, tx *sql.Tx, name string, data []byte) error {
	entries, err := decodeEntries(data)
	if err != nil {
		return err
	}
	total, err := entriesBudget(entries)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM state_entries WHERE name = ?`, name); err != nil {
		return err
	}
	statement, err := tx.PrepareContext(ctx, `INSERT INTO state_entries(name,key,value,cost) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for key, value := range entries {
		if _, err := statement.ExecContext(ctx, name, key, []byte(value), entryCost(key, value)); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE state SET value=?, entry_mode=1, entry_bytes=?, entry_count=? WHERE name=?`, []byte(`{}`), total, len(entries), name)
	return err
}

func (store *Store) saveDocument(ctx context.Context, tx *sql.Tx, name string, data []byte) error {
	var mode int
	err := tx.QueryRowContext(ctx, `SELECT entry_mode FROM state WHERE name = ?`, name).Scan(&mode)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if mode == 1 {
		return replaceEntries(ctx, tx, name, data)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO state(name,value) VALUES(?,?) ON CONFLICT(name) DO UPDATE SET value=excluded.value`, name, data)
	return err
}

func (store *Store) validateEntries(ctx context.Context) error {
	var invalid bool
	// Validate aggregate bounds before constructing logical documents from an
	// imported database. SQL counters alone are not trusted restoration input.
	err := store.db.QueryRowContext(ctx, `SELECT EXISTS(
        SELECT 1 FROM state_entries e LEFT JOIN state s ON s.name=e.name WHERE s.name IS NULL OR s.entry_mode<>1 OR e.cost < length(e.value)+length(CAST(json_quote(e.key) AS BLOB))+2 OR NOT json_valid(CAST(e.value AS TEXT))
        UNION ALL SELECT 1 FROM state s LEFT JOIN (SELECT name,count(*) AS n,sum(cost) AS bytes FROM state_entries GROUP BY name) e ON e.name=s.name
        WHERE s.entry_mode=1 AND (s.entry_count<>coalesce(e.n,0) OR s.entry_bytes<>1+coalesce(e.bytes,0))
    )`).Scan(&invalid)
	if err != nil {
		return err
	}
	if invalid {
		return errors.New("invalid persisted record storage")
	}
	return nil
}

func (store *Store) entryValue(name, key string, value any) ([]byte, error) {
	if key == "" || len(key) > 256 || !utf8.ValidString(key) {
		return nil, errors.New("invalid record key")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	document, err := json.Marshal(map[string]json.RawMessage{key: data})
	if err != nil || !store.validDocument(name, document) {
		return nil, errors.New("invalid record value")
	}

	return data, nil
}

func writeEntry(ctx context.Context, tx *sql.Tx, name, key string, data []byte) error {
	var mode int
	if err := tx.QueryRowContext(ctx, `SELECT entry_mode FROM state WHERE name = ?`, name).Scan(&mode); err != nil {
		return err
	}
	if mode != 1 {
		return errors.New("record storage is not enabled")
	}
	oldCost, increment := 0, 0
	err := tx.QueryRowContext(ctx, `SELECT cost FROM state_entries WHERE name = ? AND key = ?`, name, key).Scan(&oldCost)
	if errors.Is(err, sql.ErrNoRows) {
		increment = 1
	} else if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE state SET entry_bytes = entry_bytes + ?, entry_count = entry_count + ? WHERE name = ?`, entryCost(key, data)-oldCost, increment, name); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO state_entries(name,key,value,cost) VALUES(?,?,?,?) ON CONFLICT(name,key) DO UPDATE SET value=excluded.value,cost=excluded.cost`, name, key, data, entryCost(key, data)); err != nil {
		return err
	}
	return nil
}

func entryDocument(ctx context.Context, tx *sql.Tx, name string) ([]byte, int, error) {
	var data []byte
	var mode int
	err := tx.QueryRowContext(ctx, `SELECT value, entry_mode FROM state WHERE name = ?`, name).Scan(&data, &mode)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, 0, err
	}
	if errors.Is(err, sql.ErrNoRows) {
		data = []byte(`{}`)
		if _, err := tx.ExecContext(ctx, `INSERT INTO state(name,value) VALUES(?,?)`, name, data); err != nil {
			return nil, 0, err
		}
	}
	return data, mode, nil
}
