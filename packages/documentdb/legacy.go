package documentdb

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"unicode/utf8"
)

type legacyState struct {
	documents map[string][]byte
	entries   map[string]string
}

var errInvalidRecordEncoding = errors.New("invalid record encoding")

func (legacy legacyState) names() []string {
	names := make([]string, 0, len(legacy.documents)+len(legacy.entries))
	for name := range legacy.documents {
		names = append(names, name)
	}
	for name := range legacy.entries {
		names = append(names, name)
	}
	return names
}

func readLegacy(dataDir string, config Config, allowed map[string]struct{}) (legacyState, error) {
	store := &Store{allowed: allowed, config: config}
	legacy := legacyState{documents: make(map[string][]byte), entries: make(map[string]string)}
	for _, name := range config.Documents {
		path := filepath.Join(dataDir, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() {
			return legacyState{}, fmt.Errorf("invalid legacy state %s", name)
		}
		if info.Size() > DocumentSizeLimit && info.Size() <= EntryDocumentSizeLimit && slices.Contains(config.EntryDocuments, name) {
			if err := store.scanEntryFile(name, path, func(string, []byte) error { return nil }); err != nil {
				return legacyState{}, fmt.Errorf("invalid legacy state %s", name)
			}
			legacy.entries[name] = path
			continue
		}
		if info.Size() > DocumentSizeLimit {
			return legacyState{}, fmt.Errorf("invalid legacy state %s", name)
		}
		data, err := os.ReadFile(path)
		if err != nil || !store.validDocument(name, data) {
			return legacyState{}, fmt.Errorf("invalid legacy state %s", name)
		}
		legacy.documents[name] = data
	}
	return legacy, nil
}

func (store *Store) importEntries(ctx context.Context, tx *sql.Tx, name, path string) error {
	if _, err := tx.ExecContext(ctx, `INSERT INTO state(name,value) VALUES(?,?) ON CONFLICT(name) DO NOTHING`, name, []byte(`{}`)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM state_entries WHERE name = ?`, name); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE state SET value=?, entry_mode=1, entry_bytes=1, entry_count=0 WHERE name=?`, []byte(`{}`), name); err != nil {
		return err
	}
	return store.scanEntryFile(name, path, func(key string, value []byte) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO state_entries(name,key,value,cost) VALUES(?,?,?,?)`, name, key, value, entryCost(key, value))
		return err
	})
}

func (store *Store) scanEntryFile(name, path string, emit func(string, []byte) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(&utf8Reader{reader: bufio.NewReader(io.LimitReader(file, EntryDocumentSizeLimit+1))})
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return errInvalidRecordEncoding
	}
	keys := make(map[string]struct{})
	for decoder.More() {
		key, value, err := nextEntry(decoder, len(keys))
		if err != nil {
			return err
		}
		if _, duplicate := keys[key]; duplicate {
			return errors.New("duplicate record key")
		}
		keys[key] = struct{}{}
		data, err := store.entryValue(name, key, value)
		if err != nil {
			return err
		}
		if err := emit(key, data); err != nil {
			return err
		}
	}
	if token, err := decoder.Token(); err != nil || token != json.Delim('}') {
		return errInvalidRecordEncoding
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errInvalidRecordEncoding
	}
	return nil
}

func nextEntry(decoder *json.Decoder, count int) (string, json.RawMessage, error) {
	if count >= EntryCountLimit {
		return "", nil, errors.New("record cardinality is too large")
	}
	token, err := decoder.Token()
	key, ok := token.(string)
	if err != nil || !ok || !validEntryKey(key) {
		return "", nil, errors.New("invalid record key")
	}
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil || len(value) > EntrySizeLimit {
		return "", nil, errors.New("invalid record value")
	}
	if err := uniqueJSONValue(json.NewDecoder(bytes.NewReader(value)), 1); err != nil {
		return "", nil, err
	}
	return key, value, nil
}

type utf8Reader struct {
	reader  io.Reader
	pending []byte
}

func (reader *utf8Reader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	data := make([]byte, 0, len(reader.pending)+count)
	data = append(append(data, reader.pending...), buffer[:count]...)
	complete := len(data)
	for index := len(data) - 1; index >= 0 && index >= len(data)-utf8.UTFMax+1; index-- {
		if utf8.RuneStart(data[index]) {
			if !utf8.FullRune(data[index:]) {
				complete = index
			}
			break
		}
	}
	if !utf8.Valid(data[:complete]) {
		return 0, errInvalidRecordEncoding
	}
	reader.pending = append(reader.pending[:0], data[complete:]...)
	if errors.Is(err, io.EOF) && len(reader.pending) > 0 {
		return 0, errInvalidRecordEncoding
	}
	return count, err
}
