package database

import (
	"context"

	"github.com/MikeO7/kinosail-subtitles/internal/settingsstate"
	"github.com/MikeO7/kinosail/packages/documentdb"
)

const (
	Filename               = documentdb.Filename
	SchemaVersion          = documentdb.SchemaVersion
	DocumentSizeLimit      = documentdb.DocumentSizeLimit
	EntryDocumentSizeLimit = documentdb.EntryDocumentSizeLimit
)

var Documents = append(documentdb.PlayerDocuments(), "subtitle_ledger.json", "subtitle_records.json", "subtitle_searches.json")

type (
	Store        = documentdb.Store
	Batch        = documentdb.Batch
	EntryChanges = documentdb.EntryChanges
)

func config() documentdb.Config {
	config := documentdb.PlayerConfig()
	config.Documents = Documents
	config.Validate = validateDocument
	config.ValidateBeforeMigration = true
	return config
}

func validateDocument(name string, data []byte) error {
	if name == "settings.json" {
		return settingsstate.Validate(data)
	}
	return nil
}

func Open(dataDir string, keepOpen bool) (*Store, error) {
	return documentdb.Open(dataDir, keepOpen, config())
}

func OpenContext(ctx context.Context, dataDir string, keepOpen bool) (*Store, error) {
	return documentdb.OpenContext(ctx, dataDir, keepOpen, config())
}
