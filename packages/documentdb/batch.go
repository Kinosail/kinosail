package documentdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

type EntryChanges struct {
	Set    map[string]any
	Delete []string
}

type Batch struct {
	Documents map[string]any
	Entries   map[string]EntryChanges
	Durable   bool
}

type encodedEntries struct {
	set     map[string][]byte
	deleted []string
}

type encodedBatch struct {
	documents map[string][]byte
	entries   map[string]encodedEntries
	durable   bool
}

var errInvalidBatch = errors.New("invalid state document batch")

func (store *Store) Commit(ctx context.Context, batch Batch) error {
	if store == nil {
		return nil
	}
	if ctx == nil {
		return errInvalidBatch
	}
	encoded, err := store.encodeBatch(batch)
	if err != nil {
		return err
	}
	return store.write(ctx, encoded.durable, func(tx *sql.Tx) error {
		for name, data := range encoded.documents {
			if err := store.saveDocument(ctx, tx, name, data); err != nil {
				return err
			}
		}
		for name, changes := range encoded.entries {
			if err := store.writeEntries(ctx, tx, name, changes); err != nil {
				return err
			}
		}
		return nil
	})
}

func (store *Store) encodeBatch(batch Batch) (encodedBatch, error) {
	if len(batch.Documents)+len(batch.Entries) > len(store.allowed) {
		return encodedBatch{}, errInvalidBatch
	}
	encoded := encodedBatch{documents: make(map[string][]byte, len(batch.Documents)), entries: make(map[string]encodedEntries, len(batch.Entries)), durable: batch.Durable}
	for name, value := range batch.Documents {
		data, err := json.Marshal(value)
		if err != nil {
			return encodedBatch{}, err
		}
		if !store.acceptable(name, data, DocumentSizeLimit) {
			return encodedBatch{}, fmt.Errorf("invalid state document %q", name)
		}
		encoded.documents[name] = data
		encoded.durable = encoded.durable || store.isDurable(name)
	}
	for name, changes := range batch.Entries {
		if _, duplicate := batch.Documents[name]; duplicate {
			return encodedBatch{}, errInvalidBatch
		}
		entries, err := store.encodeEntries(name, changes)
		if err != nil {
			return encodedBatch{}, err
		}
		encoded.entries[name] = entries
		encoded.durable = encoded.durable || store.isDurable(name)
	}
	return encoded, nil
}

func (store *Store) encodeEntries(name string, changes EntryChanges) (encodedEntries, error) {
	if _, ok := store.allowed[name]; !ok || len(changes.Set)+len(changes.Delete) > EntryCountLimit {
		return encodedEntries{}, errors.New("unknown record document")
	}
	encoded := encodedEntries{set: make(map[string][]byte, len(changes.Set)), deleted: changes.Delete}
	for key, value := range changes.Set {
		data, err := store.entryValue(name, key, value)
		if err != nil {
			return encodedEntries{}, err
		}
		encoded.set[key] = data
	}
	for _, key := range changes.Delete {
		if _, conflict := encoded.set[key]; conflict || !validEntryKey(key) {
			return encodedEntries{}, errors.New("invalid record key")
		}
	}
	return encoded, nil
}

func (store *Store) entryValue(name, key string, value any) ([]byte, error) {
	if !validEntryKey(key) {
		return nil, errors.New("invalid record key")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(data) > EntrySizeLimit {
		return nil, errors.New("invalid record value")
	}
	if store.config.Validate != nil {
		document := append(appendJSONString([]byte{'{'}, key), ':')
		document = append(append(document, data...), '}')
		if store.config.Validate(name, document) != nil {
			return nil, errors.New("invalid record value")
		}
	}
	return data, nil
}

func (store *Store) isDurable(name string) bool {
	_, durable := store.durable[name]
	return durable
}
