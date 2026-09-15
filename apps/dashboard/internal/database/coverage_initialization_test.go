package database

import (
	"strings"
	"testing"

	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
)

func TestCoverageSQLiteInitializationAuthorizationFailures(t *testing.T) {
	for _, test := range []struct {
		pragma string
		want   string
	}{
		{"journal_mode", "initialize SQLite:"},
		{"quick_check", "SQLite integrity check failed:"},
	} {
		t.Run(test.pragma, func(t *testing.T) {
			db, err := driver.Open(":memory:", func(conn *sqlite3.Conn) error {
				return conn.SetAuthorizer(func(action sqlite3.AuthorizerActionCode, name, _, _, _ string) sqlite3.AuthorizerReturnCode {
					if action == sqlite3.AUTH_PRAGMA && name == test.pragma {
						return sqlite3.AUTH_DENY
					}
					return sqlite3.AUTH_OK
				})
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			store := &Store{db: db}
			if err := store.initialize(t.Context()); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("initialization failure = %v, want %q", err, test.want)
			}
		})
	}
}
