package documentdb

import (
	"context"
	"database/sql"
	"testing"
)

func connectionPragmas(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	result := make(map[string]string)
	for _, name := range []string{"journal_mode", "synchronous", "busy_timeout", "journal_size_limit", "temp_store", "query_only"} {
		var value string
		if err := conn.QueryRowContext(context.Background(), `PRAGMA `+name).Scan(&value); err != nil {
			t.Fatal(err)
		}
		result[name] = value
	}
	return result
}

func TestReconnectedConnectionsKeepDurabilityPolicy(t *testing.T) {
	store, err := Open(t.TempDir(), false, Config{Documents: []string{"progress.json", "sessions.json"}, DurableDocuments: []string{"sessions.json"}})
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	writer := map[string]string{"journal_mode": "wal", "synchronous": "1", "busy_timeout": "5000", "journal_size_limit": "67108864", "temp_store": "2", "query_only": "0"}
	reader := map[string]string{"journal_mode": "wal", "busy_timeout": "5000", "temp_store": "2", "query_only": "1"}
	for range 2 {
		if err := store.SaveJSON("sessions.json", map[string]bool{"revoked": true}); err != nil {
			t.Fatal(err)
		}
		if got := connectionPragmas(t, store.db); !equalPragmas(got, writer) {
			t.Fatalf("writer pragmas = %#v", got)
		}
		if got := connectionPragmas(t, store.reader); !equalPragmas(got, reader) {
			t.Fatalf("reader pragmas = %#v", got)
		}
	}
	if _, err := store.reader.ExecContext(context.Background(), `INSERT INTO state(name,value) VALUES('progress.json','{}')`); err == nil {
		t.Fatal("reader connection accepted a write")
	}
}

func equalPragmas(got, want map[string]string) bool {
	for name, value := range want {
		if got[name] != value {
			return false
		}
	}
	return true
}

func TestDurableDocumentsMustBeConfiguredDocuments(t *testing.T) {
	if store, err := Open(t.TempDir(), true, Config{Documents: []string{"progress.json"}, DurableDocuments: []string{"sessions.json"}}); err == nil || store != nil {
		t.Fatalf("unknown durable document accepted: %v", err)
	}
}
