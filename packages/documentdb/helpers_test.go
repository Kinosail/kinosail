package documentdb

import (
	"context"
	"database/sql"
	"net/url"
	"testing"
)

var nilContext context.Context

func openRaw(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite3", (&url.URL{Scheme: "file", Path: path, RawQuery: "_txlock=immediate"}).String())
	if err != nil {
		t.Fatal(err)
	}
	if err := raw.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	return raw
}
