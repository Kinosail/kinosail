package documentdb

import (
	"context"
	"database/sql"
	"errors"
	"unicode/utf8"
)

const (
	EntrySizeLimit         = 1 << 20
	EntryDocumentSizeLimit = 256 << 20
	EntryCountLimit        = 1_000_000
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

const recordSchema = `
CREATE TABLE state_next (
    name TEXT PRIMARY KEY NOT NULL,
    value BLOB NOT NULL CHECK(length(value) <= 16777216),
    entry_mode INTEGER NOT NULL DEFAULT 0 CHECK(entry_mode IN (0,1)),
    entry_bytes INTEGER NOT NULL DEFAULT 1 CHECK(entry_bytes BETWEEN 1 AND 268435456),
    entry_count INTEGER NOT NULL DEFAULT 0 CHECK(entry_count BETWEEN 0 AND 1000000)
) STRICT;
INSERT INTO state_next(name,value,entry_mode,entry_bytes,entry_count) SELECT name,value,entry_mode,entry_bytes,entry_count FROM state;
DROP TABLE state;
ALTER TABLE state_next RENAME TO state;
CREATE TRIGGER state_entries_inserted AFTER INSERT ON state_entries BEGIN
    UPDATE state SET entry_bytes = entry_bytes + NEW.cost, entry_count = entry_count + 1 WHERE name = NEW.name;
END;
CREATE TRIGGER state_entries_updated AFTER UPDATE ON state_entries BEGIN
    UPDATE state SET entry_bytes = entry_bytes - OLD.cost + NEW.cost WHERE name = NEW.name;
END;
CREATE TRIGGER state_entries_deleted AFTER DELETE ON state_entries BEGIN
    UPDATE state SET entry_bytes = entry_bytes - OLD.cost, entry_count = entry_count - 1 WHERE name = OLD.name;
END;
PRAGMA user_version=3;`

const (
	upsertEntrySQL = `INSERT INTO state_entries(name,key,value,cost) VALUES(?,?,?,?) ON CONFLICT(name,key) DO UPDATE SET value=excluded.value,cost=excluded.cost`
	deleteEntrySQL = `DELETE FROM state_entries WHERE name = ? AND key = ?`
)

var errEntriesDisabled = errors.New("record storage is not enabled")

func (store *Store) EnableEntries(name string) error {
	if store == nil {
		return nil
	}
	if _, ok := store.allowed[name]; !ok {
		return errors.New("unknown record document")
	}
	ctx := context.Background()
	return store.write(ctx, false, func(tx *sql.Tx) error {
		data, mode, err := entryDocument(ctx, tx, name)
		if err != nil || mode == 1 {
			return err
		}
		if !store.validDocument(name, data) {
			return errors.New("invalid record document")
		}
		return replaceEntries(ctx, tx, name, data)
	})
}

func (store *Store) SaveEntry(name, key string, value any) error {
	if store == nil {
		return nil
	}
	return store.SaveEntries(name, map[string]any{key: value}, nil)
}

func (store *Store) SaveEntries(name string, set map[string]any, deleted []string) error {
	if store == nil {
		return nil
	}
	return store.Commit(context.Background(), Batch{Entries: map[string]EntryChanges{name: {Set: set, Delete: deleted}}})
}

func entryCost(key string, data []byte) int {
	return jsonStringLength(key) + len(data) + 2
}

func validEntryKey(key string) bool {
	return key != "" && len(key) <= 256 && utf8.ValidString(key)
}

func replaceEntries(ctx context.Context, tx *sql.Tx, name string, data []byte) error {
	entries, err := decodeEntries(data)
	if err != nil {
		return err
	}
	if err := entriesBudget(entries); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM state_entries WHERE name = ?`, name); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE state SET value=?, entry_mode=1, entry_bytes=1, entry_count=0 WHERE name=?`, []byte(`{}`), name); err != nil {
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
	return nil
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

func (store *Store) writeEntries(ctx context.Context, tx *sql.Tx, name string, changes encodedEntries) error {
	var mode int
	err := tx.QueryRowContext(ctx, `SELECT entry_mode FROM state WHERE name = ?`, name).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) || err == nil && mode != 1 {
		return errEntriesDisabled
	}
	if err != nil {
		return err
	}
	remove := tx.StmtContext(ctx, store.deleteEntry)
	defer remove.Close()
	for _, key := range changes.deleted {
		if _, err := remove.ExecContext(ctx, name, key); err != nil {
			return err
		}
	}
	upsert := tx.StmtContext(ctx, store.upsertEntry)
	defer upsert.Close()
	for key, data := range changes.set {
		if _, err := upsert.ExecContext(ctx, name, key, data, entryCost(key, data)); err != nil {
			return err
		}
	}
	return nil
}

func validateEntries(ctx context.Context, query queryer) error {
	var invalid bool
	err := query.QueryRowContext(ctx, `SELECT EXISTS(
        SELECT 1 FROM state_entries e LEFT JOIN state s ON s.name=e.name WHERE s.name IS NULL OR s.entry_mode<>1 OR e.cost < length(e.value)+length(CAST(json_quote(e.key) AS BLOB))+2
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
