package documentdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type storedDocument struct {
	name    string
	data    []byte
	entries bool
}

var errInvalidState = errors.New("database contains invalid state")

func readDocuments(ctx context.Context, query queryer, recordColumns bool) ([]storedDocument, error) {
	statement := `SELECT name, value FROM state ORDER BY name`
	if recordColumns {
		statement = `SELECT name, value, entry_mode, entry_bytes FROM state ORDER BY name`
	}
	rows, err := query.QueryContext(ctx, statement)
	if err != nil {
		return nil, err
	}
	documents, sizes, err := scanDocuments(rows, recordColumns)
	if err != nil {
		return nil, err
	}
	for index := range documents {
		if documents[index].entries {
			if documents[index].data, err = assembleEntries(ctx, query, documents[index].name, sizes[index]); err != nil {
				return nil, err
			}
		}
	}
	return documents, nil
}

func scanDocuments(rows *sql.Rows, recordColumns bool) ([]storedDocument, []int64, error) {
	defer rows.Close()
	var documents []storedDocument
	var sizes []int64
	for rows.Next() {
		var document storedDocument
		var mode, size int64
		destinations := []any{&document.name, &document.data}
		if recordColumns {
			destinations = append(destinations, &mode, &size)
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, nil, errInvalidState
		}
		document.entries = mode == 1
		documents = append(documents, document)
		sizes = append(sizes, size)
	}
	return documents, sizes, rows.Err()
}

func readDocument(ctx context.Context, query queryer, name string) (data []byte, entries, found bool, err error) {
	var mode, size int64
	err = query.QueryRowContext(ctx, `SELECT value, entry_mode, entry_bytes FROM state WHERE name = ?`, name).Scan(&data, &mode, &size)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, false, false, nil
	case err != nil:
		return nil, false, false, err
	case mode != 1:
		return data, false, true, nil
	}
	if data, err = assembleEntries(ctx, query, name, size); err != nil {
		return nil, true, false, err
	}
	return data, true, true, nil
}

func assembleEntries(ctx context.Context, query queryer, name string, size int64) ([]byte, error) {
	if size < 1 || size > EntryDocumentSizeLimit {
		return nil, errInvalidState
	}
	rows, err := query.QueryContext(ctx, `SELECT key, value FROM state_entries WHERE name = ? ORDER BY key`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	document := make([]byte, 1, size+1)
	document[0] = '{'
	for rows.Next() {
		var key string
		var value sql.RawBytes
		if err := rows.Scan(&key, &value); err != nil || !utf8.ValidString(key) || !json.Valid(value) {
			return nil, errInvalidState
		}
		if len(document) > 1 {
			document = append(document, ',')
		}
		document = append(appendJSONString(document, key), ':')
		document = append(document, value...)
		if len(document) >= EntryDocumentSizeLimit {
			return nil, errInvalidState
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return append(document, '}'), nil
}

func appendJSONString(buffer []byte, value string) []byte {
	const hex = "0123456789abcdef"
	buffer = append(buffer, '"')
	for index := range len(value) {
		character := value[index]
		switch {
		case character == '"' || character == '\\':
			buffer = append(buffer, '\\', character)
		case character < 0x20:
			buffer = append(buffer, '\\', 'u', '0', '0', hex[character>>4], hex[character&0xf])
		default:
			buffer = append(buffer, character)
		}
	}
	return append(buffer, '"')
}

func jsonStringLength(value string) int {
	length := 2
	for index := range len(value) {
		switch character := value[index]; {
		case character == '"' || character == '\\':
			length += 2
		case character < 0x20:
			length += 6
		default:
			length++
		}
	}
	return length
}
