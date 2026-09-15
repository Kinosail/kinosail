// Package database adapts Player's state vocabulary to the shared document store.
package database

import (
	"context"

	"github.com/MikeO7/kinosail/packages/documentdb"
)

const (
	Filename          = documentdb.Filename
	SchemaVersion     = documentdb.SchemaVersion
	DocumentSizeLimit = documentdb.DocumentSizeLimit
)

// Documents are the legacy files whose contents move into SQLite.
var Documents = documentdb.PlayerDocuments()

type Store = documentdb.Store

func config() documentdb.Config {
	config := documentdb.PlayerConfig()
	config.Documents = Documents
	return config
}

// Open initializes the database and atomically imports legacy JSON files.
func Open(dataDir string, keepOpen bool) (*Store, error) {
	return documentdb.Open(dataDir, keepOpen, config())
}

// OpenContext initializes the database within the application lifecycle.
func OpenContext(ctx context.Context, dataDir string, keepOpen bool) (*Store, error) {
	return documentdb.OpenContext(ctx, dataDir, keepOpen, config())
}
