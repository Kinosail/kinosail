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

var Documents = documentdb.PlayerDocuments()

type Store = documentdb.Store

func config() documentdb.Config {
	config := documentdb.PlayerConfig()
	config.Documents = Documents
	return config
}

func Open(dataDir string, keepOpen bool) (*Store, error) {
	return documentdb.Open(dataDir, keepOpen, config())
}

func OpenContext(ctx context.Context, dataDir string, keepOpen bool) (*Store, error) {
	return documentdb.OpenContext(ctx, dataDir, keepOpen, config())
}
