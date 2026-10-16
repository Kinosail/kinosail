package documentdb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log/slog"
	"net/url"
)

const readerConnections = 2

var (
	writerPragmas = []string{"busy_timeout(5000)", "journal_mode(WAL)", "synchronous(NORMAL)", "journal_size_limit(67108864)", "temp_store(MEMORY)"}
	readerPragmas = []string{"busy_timeout(5000)", "query_only(1)", "temp_store(MEMORY)"}
)

func connectionDSN(path string, pragmas []string, txlock string) string {
	query := url.Values{"_pragma": pragmas}
	if txlock != "" {
		query.Set("_txlock", txlock)
	}
	return (&url.URL{Scheme: "file", Path: path, RawQuery: query.Encode()}).String()
}

func openDatabase(path string, keepOpen bool, dependencies dependencies) (*sql.DB, error) {
	db, err := dependencies.openDB("sqlite3", connectionDSN(path, writerPragmas, "immediate"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(idleConnections(keepOpen, 1))
	return db, nil
}

func openReader(path string, keepOpen bool, dependencies dependencies) (*sql.DB, error) {
	db, err := dependencies.openDB("sqlite3", connectionDSN(path, readerPragmas, ""))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(readerConnections)
	db.SetMaxIdleConns(idleConnections(keepOpen, readerConnections))
	return db, nil
}

func idleConnections(keepOpen bool, connections int) int {
	if keepOpen {
		return connections
	}
	return 0
}

func (store *Store) write(ctx context.Context, durable bool, apply func(*sql.Tx) error) error {
	if !durable {
		return commit(ctx, store.db.BeginTx, apply)
	}
	conn, err := store.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `PRAGMA synchronous=FULL`); err != nil {
		return err
	}
	err = commit(ctx, conn.BeginTx, apply)
	if _, resetErr := conn.ExecContext(context.WithoutCancel(ctx), `PRAGMA synchronous=NORMAL`); resetErr != nil {
		slog.Warn("State database connection reset failed", "operation", "documentdb.durable_commit", "outcome", "connection-discarded")
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
	}
	return err
}

func commit(ctx context.Context, begin func(context.Context, *sql.TxOptions) (*sql.Tx, error), apply func(*sql.Tx) error) error {
	tx, err := begin(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := apply(tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (store *Store) read(ctx context.Context, apply func(*sql.Tx) error) error {
	tx, err := store.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	return apply(tx)
}
